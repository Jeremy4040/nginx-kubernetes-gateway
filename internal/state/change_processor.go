package state

import (
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	discoveryV1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/gateway-api/apis/v1beta1"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . ChangeProcessor

// ChangeProcessor processes the changes to resources producing the internal representation of the Gateway configuration.
// ChangeProcessor only supports one GatewayClass resource.
type ChangeProcessor interface {
	// CaptureUpsertChange captures an upsert change to a resource.
	// It panics if the resource is of unsupported type or if the passed Gateway is different from the one this ChangeProcessor
	// was created for.
	CaptureUpsertChange(obj client.Object)
	// CaptureDeleteChange captures a delete change to a resource.
	// The method panics if the resource is of unsupported type or if the passed Gateway is different from the one this ChangeProcessor
	// was created for.
	CaptureDeleteChange(resourceType client.Object, nsname types.NamespacedName)
	// Process processes any captured changes and produces an internal representation of the Gateway configuration and
	// the status information about the processed resources.
	// If no changes were captured, the changed return argument will be false and both the configuration and statuses
	// will be empty.
	Process() (changed bool, conf Configuration, statuses Statuses)
}

// ChangeProcessorConfig holds configuration parameters for ChangeProcessorImpl.
type ChangeProcessorConfig struct {
	// GatewayCtlrName is the name of the Gateway controller.
	GatewayCtlrName string
	// GatewayClassName is the name of the GatewayClass resource.
	GatewayClassName string
	// SecretMemoryManager is the secret memory manager.
	SecretMemoryManager SecretDiskMemoryManager
	// ServiceStore is the service store.
	ServiceStore ServiceStore
	// Logger is the logger for this Change Processor.
	Logger logr.Logger
}

// ChangeProcessorImpl is an implementation of ChangeProcessor.
type ChangeProcessorImpl struct {
	store *store
	// storeChanged tells if the store is changed.
	// The store is considered changed if:
	// (1) Any of its resources was deleted.
	// (2) A new resource was upserted.
	// (3) An existing resource with the updated Generation was upserted.
	storeChanged bool
	cfg          ChangeProcessorConfig

	lock sync.Mutex
}

// NewChangeProcessorImpl creates a new ChangeProcessorImpl for the Gateway resource with the configured namespace name.
func NewChangeProcessorImpl(cfg ChangeProcessorConfig) *ChangeProcessorImpl {
	return &ChangeProcessorImpl{
		store: newStore(),
		cfg:   cfg,
	}
}

// FIXME(pleshakov)
// Currently, changes (upserts/delete) trigger rebuilding of the configuration, even if the change doesn't change
// the configuration or the statuses of the resources. For example, a change in a Gateway resource that doesn't
// belong to the NGINX Gateway or an HTTPRoute that doesn't belong to any of the Gateways of the NGINX Gateway.
// Find a way to ignore changes that don't affect the configuration and/or statuses of the resources.

func (c *ChangeProcessorImpl) CaptureUpsertChange(obj client.Object) {
	c.lock.Lock()
	defer c.lock.Unlock()

	resourceChanged := true

	switch o := obj.(type) {
	case *v1beta1.GatewayClass:
		resourceChanged = c.captureGatewayClassChange(o)
	case *v1beta1.Gateway:
		resourceChanged = c.captureGatewayChange(o)
	case *v1beta1.HTTPRoute:
		resourceChanged = c.captureHTTPRouteChange(o)
	case *v1.Service:
		resourceChanged = c.captureServiceChange(o)
	case *discoveryV1.EndpointSlice:
		resourceChanged = c.captureEndpointSliceChange(o)
	default:
		panic(fmt.Errorf("ChangeProcessor doesn't support %T", obj))
	}

	c.storeChanged = c.storeChanged || resourceChanged
}

func (c *ChangeProcessorImpl) captureGatewayClassChange(gc *v1beta1.GatewayClass) bool {
	resourceChanged := true

	if gc.Name != c.cfg.GatewayClassName {
		panic(fmt.Errorf("gatewayclass resource must be %s, got %s", c.cfg.GatewayClassName, gc.Name))
	}

	// if the resource spec hasn't changed (its generation is the same), ignore the upsert
	if c.store.gc != nil && c.store.gc.Generation == gc.Generation {
		resourceChanged = false
	}

	c.store.gc = gc

	return resourceChanged
}

func (c *ChangeProcessorImpl) captureGatewayChange(gw *v1beta1.Gateway) bool {
	resourceChanged := true
	// if the resource spec hasn't changed (its generation is the same), ignore the upsert
	prev, exist := c.store.gateways[getNamespacedName(gw)]
	if exist && gw.Generation == prev.Generation {
		resourceChanged = false
	}
	c.store.gateways[getNamespacedName(gw)] = gw

	return resourceChanged
}

func (c *ChangeProcessorImpl) captureHTTPRouteChange(hr *v1beta1.HTTPRoute) bool {
	resourceChanged := true

	// if the resource spec hasn't changed (its generation is the same), ignore the upsert
	prev, exist := c.store.httpRoutes[getNamespacedName(hr)]
	if exist && hr.Generation == prev.Generation {
		resourceChanged = false
	}
	c.store.httpRoutes[getNamespacedName(hr)] = hr
	c.updateServicesMap(hr)

	return resourceChanged
}

func (c *ChangeProcessorImpl) captureServiceChange(svc *v1.Service) bool {
	// We only need to trigger an update when the service exists in the store.
	_, exist := c.store.services[getNamespacedName(svc)]

	return exist
}

func (c *ChangeProcessorImpl) captureEndpointSliceChange(es *discoveryV1.EndpointSlice) bool {
	if c.updateNeededForEndpointSlice(es) {
		c.store.endpointSlices[getNamespacedName(es)] = es

		return true
	}

	return false
}

func (c *ChangeProcessorImpl) updateServicesMap(hr *v1beta1.HTTPRoute) {
	svcNames := getBackendServiceNamesFromRoute(hr)

	for _, svcNsname := range svcNames {
		existingRoutesForSvc, exist := c.store.services[svcNsname]
		if !exist {
			c.store.services[svcNsname] = map[types.NamespacedName]struct{}{getNamespacedName(hr): {}}
			continue
		}

		existingRoutesForSvc[getNamespacedName(hr)] = struct{}{}
	}
}

// We only need to update the config if the endpoint slice is owned by a service we have in the store.
func (c *ChangeProcessorImpl) updateNeededForEndpointSlice(endpointSlice *discoveryV1.EndpointSlice) bool {
	for _, ownerRef := range endpointSlice.OwnerReferences {

		if ownerRef.Kind != "Service" {
			continue
		}

		svcNsname := types.NamespacedName{
			Namespace: endpointSlice.Namespace,
			Name:      ownerRef.Name,
		}

		if _, exist := c.store.services[svcNsname]; exist {
			return true
		}
	}

	return false
}

func (c *ChangeProcessorImpl) removeRouteFromServicesMap(hr *v1beta1.HTTPRoute) {
	backendServiceNames := getBackendServiceNamesFromRoute(hr)
	for _, svcName := range backendServiceNames {
		routesForSvc, exist := c.store.services[svcName]
		if exist {
			delete(routesForSvc, getNamespacedName(hr))
			if len(routesForSvc) == 0 {
				delete(c.store.services, svcName)
			}
		}
	}
}

func (c *ChangeProcessorImpl) CaptureDeleteChange(resourceType client.Object, nsname types.NamespacedName) {
	c.lock.Lock()
	defer c.lock.Unlock()

	resourceChanged := true

	switch resourceType.(type) {
	case *v1beta1.GatewayClass:
		if nsname.Name != c.cfg.GatewayClassName {
			panic(fmt.Errorf("gatewayclass resource must be %s, got %s", c.cfg.GatewayClassName, nsname.Name))
		}
		c.store.gc = nil
	case *v1beta1.Gateway:
		delete(c.store.gateways, nsname)
	case *v1beta1.HTTPRoute:
		if r, exists := c.store.httpRoutes[nsname]; exists {
			c.removeRouteFromServicesMap(r)
		}
		delete(c.store.httpRoutes, nsname)
	case *v1.Service:
		// We only need to trigger an update when the service exists in the store.
		if _, exist := c.store.services[nsname]; !exist {
			resourceChanged = false
		}
	case *discoveryV1.EndpointSlice:
		es, exist := c.store.endpointSlices[nsname]
		resourceChanged = exist && c.updateNeededForEndpointSlice(es)

		delete(c.store.endpointSlices, nsname)
	default:
		panic(fmt.Errorf("ChangeProcessor doesn't support %T", resourceType))
	}

	c.storeChanged = c.storeChanged || resourceChanged
}

func (c *ChangeProcessorImpl) Process() (changed bool, conf Configuration, statuses Statuses) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if !c.storeChanged {
		return false, conf, statuses
	}

	c.storeChanged = false

	graph, warnings := buildGraph(
		c.store,
		c.cfg.GatewayCtlrName,
		c.cfg.GatewayClassName,
		c.cfg.SecretMemoryManager,
		c.cfg.ServiceStore,
	)

	for obj, objWarnings := range warnings {
		for _, w := range objWarnings {
			// FIXME(pleshakov): report warnings via Object status
			c.cfg.Logger.Info("Got warning while building graph",
				"kind", obj.GetObjectKind().GroupVersionKind().Kind,
				"namespace", obj.GetNamespace(),
				"name", obj.GetName(),
				"warning", w)
		}
	}

	conf = buildConfiguration(graph)
	statuses = buildStatuses(graph)

	return true, conf, statuses
}

// FIXME(pleshakov): for now, we only support a single backend reference
func getBackendServiceNamesFromRoute(hr *v1beta1.HTTPRoute) []types.NamespacedName {
	svcNames := make([]types.NamespacedName, 0, len(hr.Spec.Rules))

	for _, rule := range hr.Spec.Rules {
		if len(rule.BackendRefs) == 0 {
			continue
		}
		ref := rule.BackendRefs[0].BackendRef

		if ref.Kind != nil && *ref.Kind != "Service" {
			continue
		}

		ns := hr.Namespace
		if ref.Namespace != nil {
			ns = string(*ref.Namespace)
		}

		svcNames = append(svcNames, types.NamespacedName{Namespace: ns, Name: string(ref.Name)})
	}

	return svcNames
}
