package resolver

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	discoveryV1 "k8s.io/api/discovery/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nginxinc/nginx-kubernetes-gateway/pkg/sdk"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . ServiceResolver

// ServiceResolver resolves a Service and Service Port to a list of Endpoints.
// Returns an error if the Service or Service Port cannot be resolved.
type ServiceResolver interface {
	Resolve(ctx context.Context, svc *v1.Service, svcPort int32) ([]Endpoint, error)
}

// Endpoint is the internal representation of a Kubernetes endpoint.
type Endpoint struct {
	// Address is the IP address of the endpoint.
	Address string
	// Port is the port of the endpoint.
	Port int32
}

// ServiceResolverImpl implements ServiceResolver.
type ServiceResolverImpl struct {
	client client.Client
}

// NewServiceResolverImpl creates a new instance of a ServiceResolverImpl.
func NewServiceResolverImpl(client client.Client) *ServiceResolverImpl {
	return &ServiceResolverImpl{client: client}
}

// Resolve resolves a Service and Service Port to a list of Endpoints.
// Returns an error if the Service or Service Port cannot be resolved.
func (e *ServiceResolverImpl) Resolve(ctx context.Context, svc *v1.Service, svcPort int32) ([]Endpoint, error) {
	if svc == nil {
		return nil, fmt.Errorf("cannot resolve a nil Service")
	}

	// We list EndpointSlices using the Service Name Index Field we added as an index to the EndpointSlice cache.
	// This allows us to perform a quick lookup of all EndpointSlices for a Service.
	var endpointSliceList discoveryV1.EndpointSliceList
	err := e.client.List(
		ctx,
		&endpointSliceList,
		client.MatchingFields{sdk.KubernetesServiceNameIndexField: svc.Name},
		client.InNamespace(svc.Namespace),
	)

	if err != nil || len(endpointSliceList.Items) == 0 {
		return nil, fmt.Errorf("no endpoints found for Service %s", client.ObjectKeyFromObject(svc))
	}

	return resolveEndpoints(svc, svcPort, endpointSliceList)
}

func resolveEndpoints(svc *v1.Service, svcPort int32, endpointSliceList discoveryV1.EndpointSliceList) ([]Endpoint, error) {
	targetPort, err := getTargetPort(svc, svcPort)
	if err != nil {
		return nil, err
	}

	capacity := calculateEndpointSliceCapacity(endpointSliceList.Items, targetPort)

	if capacity == 0 {
		return nil, fmt.Errorf("no valid endpoints found for Service %s and port %d", client.ObjectKeyFromObject(svc), svcPort)
	}

	endpoints := make([]Endpoint, 0, capacity)

	for _, eps := range endpointSliceList.Items {

		if ignoreEndpointSlice(eps, targetPort) {
			continue
		}

		for _, endpoint := range eps.Endpoints {

			if !endpointReady(endpoint) {
				continue
			}

			for _, address := range endpoint.Addresses {
				ep := Endpoint{Address: address, Port: targetPort}
				endpoints = append(endpoints, ep)
			}
		}
	}

	return endpoints, nil
}

func getTargetPort(svc *v1.Service, svcPort int32) (int32, error) {
	for _, port := range svc.Spec.Ports {
		if port.Port == svcPort {
			val := port.TargetPort.IntValue()
			if val == 0 {
				break
			}
			return int32(val), nil
		}
	}

	return 0, fmt.Errorf("no matching target port for Service %s/%s and port %d", svc.Namespace, svc.Name, svcPort)
}

func ignoreEndpointSlice(endpointSlice discoveryV1.EndpointSlice, targetPort int32) bool {
	return endpointSlice.AddressType != discoveryV1.AddressTypeIPv4 || !targetPortExists(endpointSlice.Ports, targetPort)
}

func calculateEndpointSliceCapacity(endpointSlices []discoveryV1.EndpointSlice, targetPort int32) (capacity int) {
	for _, es := range endpointSlices {

		if ignoreEndpointSlice(es, targetPort) {
			continue
		}

		for _, e := range es.Endpoints {
			if !endpointReady(e) {
				continue
			}
			capacity += len(e.Addresses)
		}
	}

	return
}

func endpointReady(endpoint discoveryV1.Endpoint) bool {
	ready := endpoint.Conditions.Ready
	return ready != nil && *ready
}

func targetPortExists(ports []discoveryV1.EndpointPort, targetPort int32) bool {
	for _, port := range ports {
		if port.Port == nil {
			continue
		}

		if *port.Port == targetPort {
			return true
		}
	}

	return false
}
