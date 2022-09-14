package resolver

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	discoveryV1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/nginxinc/nginx-kubernetes-gateway/internal/helpers"
)

func TestCalculateEndpointSliceCapacity(t *testing.T) {
	addresses := []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}

	readyEndpoint1 := discoveryV1.Endpoint{
		Addresses:  addresses,
		Conditions: discoveryV1.EndpointConditions{Ready: helpers.GetBoolPointer(true)},
	}

	notReadyEndpoint := discoveryV1.Endpoint{
		Addresses:  addresses,
		Conditions: discoveryV1.EndpointConditions{Ready: helpers.GetBoolPointer(false)},
	}

	validEndpointSlice := discoveryV1.EndpointSlice{
		AddressType: discoveryV1.AddressTypeIPv4,
		Endpoints:   []discoveryV1.Endpoint{readyEndpoint1, readyEndpoint1, readyEndpoint1}, // in reality these endpoints would be different but for this test it doesn't matter
		Ports: []discoveryV1.EndpointPort{
			{
				Port: helpers.GetInt32Pointer(80),
			},
			{
				Port: helpers.GetInt32Pointer(443),
			},
		},
	}

	invalidAddressTypeEndpointSlice := discoveryV1.EndpointSlice{
		AddressType: discoveryV1.AddressTypeIPv6,
		Endpoints:   []discoveryV1.Endpoint{readyEndpoint1},
		Ports: []discoveryV1.EndpointPort{
			{
				Port: helpers.GetInt32Pointer(80),
			},
		},
	}

	invalidPortEndpointSlice := discoveryV1.EndpointSlice{
		AddressType: discoveryV1.AddressTypeIPv4,
		Endpoints:   []discoveryV1.Endpoint{readyEndpoint1},
		Ports: []discoveryV1.EndpointPort{
			{
				Port: helpers.GetInt32Pointer(8080),
			},
		},
	}

	notReadyEndpointSlice := discoveryV1.EndpointSlice{
		AddressType: discoveryV1.AddressTypeIPv4,
		Endpoints:   []discoveryV1.Endpoint{notReadyEndpoint, notReadyEndpoint}, // in reality these endpoints would be different but for this test it doesn't matter
		Ports: []discoveryV1.EndpointPort{
			{
				Port: helpers.GetInt32Pointer(80),
			},
			{
				Port: helpers.GetInt32Pointer(443),
			},
		},
	}

	mixedValidityEndpointSlice := discoveryV1.EndpointSlice{
		AddressType: discoveryV1.AddressTypeIPv4,
		Endpoints:   []discoveryV1.Endpoint{readyEndpoint1, notReadyEndpoint, readyEndpoint1}, // 6 valid endpoints
		Ports: []discoveryV1.EndpointPort{
			{
				Port: helpers.GetInt32Pointer(80),
			},
		},
	}

	testcases := []struct {
		msg            string
		endpointSlices []discoveryV1.EndpointSlice
		targetPort     int32
		expCapacity    int
	}{
		{
			msg: "multiple endpoint slices - multiple valid endpoints",
			endpointSlices: []discoveryV1.EndpointSlice{
				validEndpointSlice,
				validEndpointSlice, // in reality these endpoints would be different but for this test it doesn't matter
			},
			targetPort:  80,
			expCapacity: 18,
		},
		{
			msg: "multiple endpoint slices - some valid ",
			endpointSlices: []discoveryV1.EndpointSlice{
				validEndpointSlice,
				invalidAddressTypeEndpointSlice,
				validEndpointSlice,
				invalidPortEndpointSlice,
			},
			targetPort:  80,
			expCapacity: 18,
		},
		{
			msg:            "multiple endpoints - some valid ",
			endpointSlices: []discoveryV1.EndpointSlice{mixedValidityEndpointSlice},
			targetPort:     80,
			expCapacity:    6,
		},
		{
			msg:            "multiple endpoint slices - all invalid ",
			endpointSlices: []discoveryV1.EndpointSlice{invalidAddressTypeEndpointSlice, invalidPortEndpointSlice},
			targetPort:     80,
			expCapacity:    0,
		},
		{
			msg:            "multiple endpoints - all invalid ",
			endpointSlices: []discoveryV1.EndpointSlice{notReadyEndpointSlice},
			targetPort:     80,
			expCapacity:    0,
		},
	}

	for _, tc := range testcases {
		capacity := calculateEndpointSliceCapacity(tc.endpointSlices, tc.targetPort)
		if capacity != tc.expCapacity {
			t.Errorf("calculateEndpointSliceCapacity() mismatch for %q; expected %d, got %d", tc.msg, capacity, tc.expCapacity)
		}
	}
}

func TestGetTargetPort(t *testing.T) {
	testcases := []struct {
		msg     string
		svc     *v1.Service
		svcPort int32
		expPort int32
		expErr  bool
	}{
		{
			msg: "int target port",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Port:       443,
							TargetPort: intstr.FromInt(8443),
						},
						{
							Port:       80,
							TargetPort: intstr.FromInt(8080),
						},
					},
				},
			},
			svcPort: 80,
			expPort: 8080,
			expErr:  false,
		},
		{
			msg: "string target port",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Port:       443,
							TargetPort: intstr.FromInt(8443),
						},
						{
							Port:       80,
							TargetPort: intstr.FromString("8080"),
						},
					},
				},
			},
			svcPort: 80,
			expPort: 8080,
			expErr:  false,
		},
		{
			msg: "no target port",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Port: 80,
						},
					},
				},
			},
			svcPort: 80,
			expPort: 0,
			expErr:  true,
		},
		{
			msg: "no matching target port",
			svc: &v1.Service{
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Port:       443,
							TargetPort: intstr.FromInt(8443),
						},
						{
							Port:       80,
							TargetPort: intstr.FromInt(8080),
						},
					},
				},
			},
			svcPort: 90,
			expPort: 0,
			expErr:  true,
		},
	}
	for _, tc := range testcases {
		port, err := getTargetPort(tc.svc, tc.svcPort)
		if tc.expErr && err == nil {
			t.Errorf("getTargetPort() did not return an error for %q", tc.msg)
		}
		if !tc.expErr && err != nil {
			t.Errorf("getTargetPort() returned an error for %q", tc.msg)
		}
		if tc.expPort != port {
			t.Errorf("getTargetPort() mismatch on port for %q; expected %d, got %d", tc.msg, tc.expPort, port)
		}
	}
}

func TestIgnoreEndpointSlice(t *testing.T) {
	var port int32 = 4000

	testcases := []struct {
		msg        string
		slice      discoveryV1.EndpointSlice
		targetPort int32
		ignore     bool
	}{
		{
			msg: "IPV6 address type",
			slice: discoveryV1.EndpointSlice{
				AddressType: discoveryV1.AddressTypeIPv6,
			},
			targetPort: 8080,
			ignore:     true,
		},
		{
			msg: "FQDN address type",
			slice: discoveryV1.EndpointSlice{
				AddressType: discoveryV1.AddressTypeFQDN,
			},
			targetPort: 8080,
			ignore:     true,
		},
		{
			msg: "no matching target port",
			slice: discoveryV1.EndpointSlice{
				AddressType: discoveryV1.AddressTypeIPv4,
				Ports: []discoveryV1.EndpointPort{
					{
						Port: &port,
					},
				},
			},
			targetPort: 8080,
			ignore:     true,
		},
		{
			msg: "normal",
			slice: discoveryV1.EndpointSlice{
				AddressType: discoveryV1.AddressTypeIPv4,
				Ports: []discoveryV1.EndpointPort{
					{
						Port: &port,
					},
				},
			},
			targetPort: 4000,
			ignore:     false,
		},
	}
	for _, tc := range testcases {
		if ignoreEndpointSlice(tc.slice, tc.targetPort) != tc.ignore {
			t.Errorf("ignoreEndpointSlice() mismatch for %q; expected %t", tc.msg, tc.ignore)
		}
	}
}

func TestEndpointReady(t *testing.T) {
	testcases := []struct {
		msg      string
		endpoint discoveryV1.Endpoint
		ready    bool
	}{
		{
			msg: "endpoint ready",
			endpoint: discoveryV1.Endpoint{
				Conditions: discoveryV1.EndpointConditions{
					Ready: helpers.GetBoolPointer(true),
				},
			},
			ready: true,
		},
		{
			msg: "nil ready",
			endpoint: discoveryV1.Endpoint{
				Conditions: discoveryV1.EndpointConditions{
					Ready: nil,
				},
			},
			ready: false,
		},
		{
			msg: "endpoint not ready",
			endpoint: discoveryV1.Endpoint{
				Conditions: discoveryV1.EndpointConditions{
					Ready: helpers.GetBoolPointer(false),
				},
			},
			ready: false,
		},
	}
	for _, tc := range testcases {
		if endpointReady(tc.endpoint) != tc.ready {
			t.Errorf("endpointReady() mismatch for %q; expected %t", tc.msg, tc.ready)
		}
	}
}

func TestTargetPortExists(t *testing.T) {
	testcases := []struct {
		msg        string
		ports      []discoveryV1.EndpointPort
		targetPort int32
		exists     bool
	}{
		{
			msg: "nil port",
			ports: []discoveryV1.EndpointPort{
				{
					Port: nil,
				},
			},
			targetPort: 8080,
			exists:     false,
		},
		{
			msg: "no matching targetPort",
			ports: []discoveryV1.EndpointPort{
				{
					Port: helpers.GetInt32Pointer(80),
				},
				{
					Port: helpers.GetInt32Pointer(81),
				},
				{
					Port: helpers.GetInt32Pointer(82),
				},
			},
			targetPort: 8080,
			exists:     false,
		},
		{
			msg: "matching targetPort",
			ports: []discoveryV1.EndpointPort{
				{
					Port: helpers.GetInt32Pointer(80),
				},
				{
					Port: helpers.GetInt32Pointer(81),
				},
				{
					Port: helpers.GetInt32Pointer(8080),
				},
			},
			targetPort: 8080,
			exists:     true,
		},
	}
	for _, tc := range testcases {
		if targetPortExists(tc.ports, tc.targetPort) != tc.exists {
			t.Errorf("targetPortExists() mismatch on %q; expected %t", tc.msg, tc.exists)
		}
	}
}
