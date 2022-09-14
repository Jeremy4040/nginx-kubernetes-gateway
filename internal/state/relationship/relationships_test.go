package relationship

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/nginxinc/nginx-kubernetes-gateway/internal/helpers"
)

func TestGetBackendServiceNamesFromRoute(t *testing.T) {
	getNormalRefs := func(svcName v1beta1.ObjectName) []v1beta1.HTTPBackendRef {
		return []v1beta1.HTTPBackendRef{
			{
				BackendRef: v1beta1.BackendRef{
					BackendObjectReference: v1beta1.BackendObjectReference{
						Kind:      (*v1beta1.Kind)(helpers.GetStringPointer("Service")),
						Name:      svcName,
						Namespace: (*v1beta1.Namespace)(helpers.GetStringPointer("test")),
						Port:      (*v1beta1.PortNumber)(helpers.GetInt32Pointer(80)),
					},
				},
			},
		}
	}

	getModifiedRefs := func(svcName v1beta1.ObjectName, mod func([]v1beta1.HTTPBackendRef) []v1beta1.HTTPBackendRef) []v1beta1.HTTPBackendRef {
		return mod(getNormalRefs(svcName))
	}

	hr := &v1beta1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test"},
		Spec: v1beta1.HTTPRouteSpec{
			Rules: []v1beta1.HTTPRouteRule{
				{
					BackendRefs: getNormalRefs("svc1"),
				},
				{
					BackendRefs: getNormalRefs("svc1"), // duplicate
				},
				{
					BackendRefs: getModifiedRefs("invalid-kind",
						func(refs []v1beta1.HTTPBackendRef) []v1beta1.HTTPBackendRef {
							refs[0].Kind = (*v1beta1.Kind)(helpers.GetStringPointer("Invalid"))
							return refs
						},
					),
				},
				{
					BackendRefs: getModifiedRefs("nil-namespace",
						func(refs []v1beta1.HTTPBackendRef) []v1beta1.HTTPBackendRef {
							refs[0].Namespace = nil
							return refs
						},
					),
				},
				{
					BackendRefs: getModifiedRefs("diff-namespace",
						func(refs []v1beta1.HTTPBackendRef) []v1beta1.HTTPBackendRef {
							refs[0].Namespace = (*v1beta1.Namespace)(helpers.GetStringPointer("not-test"))
							return refs
						},
					),
				},
				{
					BackendRefs: nil,
				},
				{
					BackendRefs: getNormalRefs("svc2"),
				},
			},
		},
	}

	expNames := map[types.NamespacedName]struct{}{
		{Namespace: "test", Name: "svc1"}:               {},
		{Namespace: "test", Name: "nil-namespace"}:      {},
		{Namespace: "not-test", Name: "diff-namespace"}: {},
		{Namespace: "test", Name: "svc2"}:               {},
	}
	names := getBackendServiceNamesFromRoute(hr)
	if diff := cmp.Diff(expNames, names); diff != "" {
		t.Errorf("getBackendServiceNamesFromRoute() mismatch (-want +got):\n%s", diff)
	}
}
