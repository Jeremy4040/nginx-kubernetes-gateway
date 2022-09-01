package state

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/nginxinc/nginx-kubernetes-gateway/internal/helpers"
)

func TestBuildConfiguration(t *testing.T) {
	createRoute := func(name string, hostname string, listenerName string, paths ...string) *v1beta1.HTTPRoute {
		rules := make([]v1beta1.HTTPRouteRule, 0, len(paths))
		for _, p := range paths {
			rules = append(rules, v1beta1.HTTPRouteRule{
				Matches: []v1beta1.HTTPRouteMatch{
					{
						Path: &v1beta1.HTTPPathMatch{
							Value: helpers.GetStringPointer(p),
						},
					},
				},
			})
		}
		return &v1beta1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      name,
			},
			Spec: v1beta1.HTTPRouteSpec{
				CommonRouteSpec: v1beta1.CommonRouteSpec{
					ParentRefs: []v1beta1.ParentReference{
						{
							Namespace:   (*v1beta1.Namespace)(helpers.GetStringPointer("test")),
							Name:        "gateway",
							SectionName: (*v1beta1.SectionName)(helpers.GetStringPointer(listenerName)),
						},
					},
				},
				Hostnames: []v1beta1.Hostname{
					v1beta1.Hostname(hostname),
				},
				Rules: rules,
			},
		}
	}

	fooBackendSvc := backendService{name: "foo", namespace: "test", port: 80}

	fooBackend := backend{
		Endpoints: []Endpoint{
			{
				Address: "10.0.0.0",
				Port:    8080,
			},
		},
	}

	fooUpstreamName := "test_foo_80"

	fooUpstream := Upstream{
		Name: fooUpstreamName,
		Endpoints: []Endpoint{
			{
				Address: "10.0.0.0",
				Port:    8080,
			},
		},
	}

	hr1 := createRoute("hr-1", "foo.example.com", "listener-80-1", "/")

	routeHR1 := &route{
		Source: hr1,
		ValidSectionNameRefs: map[string]struct{}{
			"listener-80-1": {},
		},
		InvalidSectionNameRefs: map[string]struct{}{},
		BackendServices: map[ruleIndex]backendService{
			ruleIndex(0): fooBackendSvc,
		},
	}

	hr2 := createRoute("hr-2", "bar.example.com", "listener-80-1", "/")

	routeHR2 := &route{
		Source: hr2,
		ValidSectionNameRefs: map[string]struct{}{
			"listener-80-1": {},
		},
		InvalidSectionNameRefs: map[string]struct{}{},
		BackendServices: map[ruleIndex]backendService{
			ruleIndex(0): fooBackendSvc,
		},
	}

	httpsHR1 := createRoute("https-hr-1", "foo.example.com", "listener-443-1", "/")

	httpsRouteHR1 := &route{
		Source: httpsHR1,
		ValidSectionNameRefs: map[string]struct{}{
			"listener-443-1": {},
		},
		InvalidSectionNameRefs: map[string]struct{}{},
		BackendServices: map[ruleIndex]backendService{
			ruleIndex(0): fooBackendSvc,
		},
	}

	httpsHR2 := createRoute("https-hr-2", "bar.example.com", "listener-443-1", "/")

	httpsRouteHR2 := &route{
		Source: httpsHR2,
		ValidSectionNameRefs: map[string]struct{}{
			"listener-443-1": {},
		},
		InvalidSectionNameRefs: map[string]struct{}{},
		BackendServices: map[ruleIndex]backendService{
			ruleIndex(0): fooBackendSvc,
		},
	}

	hr3 := createRoute("hr-3", "foo.example.com", "listener-80-1", "/", "/third")

	routeHR3 := &route{
		Source: hr3,
		ValidSectionNameRefs: map[string]struct{}{
			"listener-80-1": {},
		},
		InvalidSectionNameRefs: map[string]struct{}{},
		BackendServices: map[ruleIndex]backendService{
			ruleIndex(0): fooBackendSvc,
			ruleIndex(1): fooBackendSvc,
		},
	}

	httpsHR3 := createRoute("https-hr-3", "foo.example.com", "listener-443-1", "/", "/third")

	httpsRouteHR3 := &route{
		Source: httpsHR3,
		ValidSectionNameRefs: map[string]struct{}{
			"listener-443-1": {},
		},
		InvalidSectionNameRefs: map[string]struct{}{},
		BackendServices: map[ruleIndex]backendService{
			ruleIndex(0): fooBackendSvc,
			ruleIndex(1): fooBackendSvc,
		},
	}

	hr4 := createRoute("hr-4", "foo.example.com", "listener-80-1", "/fourth", "/")

	routeHR4 := &route{
		Source: hr4,
		ValidSectionNameRefs: map[string]struct{}{
			"listener-80-1": {},
		},
		InvalidSectionNameRefs: map[string]struct{}{},
		BackendServices: map[ruleIndex]backendService{
			ruleIndex(0): fooBackendSvc,
			ruleIndex(1): fooBackendSvc,
		},
	}

	httpsHR4 := createRoute("https-hr-4", "foo.example.com", "listener-443-1", "/fourth", "/")

	httpsRouteHR4 := &route{
		Source: httpsHR4,
		ValidSectionNameRefs: map[string]struct{}{
			"listener-443-1": {},
		},
		InvalidSectionNameRefs: map[string]struct{}{},
		BackendServices: map[ruleIndex]backendService{
			ruleIndex(0): fooBackendSvc,
			ruleIndex(1): fooBackendSvc,
		},
	}

	httpsHR5 := createRoute("https-hr-5", "example.com", "listener-443-with-hostname", "/")

	httpsRouteHR5 := &route{
		Source: httpsHR5,
		ValidSectionNameRefs: map[string]struct{}{
			"listener-443-with-hostname": {},
		},
		InvalidSectionNameRefs: map[string]struct{}{},
		BackendServices: map[ruleIndex]backendService{
			ruleIndex(0): {}, // invalid upstream
		},
	}

	listener80 := v1beta1.Listener{
		Name:     "listener-80-1",
		Hostname: nil,
		Port:     80,
		Protocol: v1beta1.HTTPProtocolType,
	}

	listener443 := v1beta1.Listener{
		Name:     "listener-443-1",
		Hostname: nil,
		Port:     443,
		Protocol: v1beta1.HTTPSProtocolType,
		TLS: &v1beta1.GatewayTLSConfig{
			Mode: helpers.GetTLSModePointer(v1beta1.TLSModeTerminate),
			CertificateRefs: []v1beta1.SecretObjectReference{
				{
					Kind:      (*v1beta1.Kind)(helpers.GetStringPointer("Secret")),
					Name:      "secret",
					Namespace: (*v1beta1.Namespace)(helpers.GetStringPointer("test")),
				},
			},
		},
	}
	hostname := v1beta1.Hostname("example.com")

	listener443WithHostname := v1beta1.Listener{
		Name:     "listener-443-with-hostname",
		Hostname: &hostname,
		Port:     443,
		Protocol: v1beta1.HTTPSProtocolType,
		TLS: &v1beta1.GatewayTLSConfig{
			Mode: helpers.GetTLSModePointer(v1beta1.TLSModeTerminate),
			CertificateRefs: []v1beta1.SecretObjectReference{
				{
					Kind:      (*v1beta1.Kind)(helpers.GetStringPointer("Secret")),
					Name:      "secret",
					Namespace: (*v1beta1.Namespace)(helpers.GetStringPointer("test")),
				},
			},
		},
	}

	invalidListener := v1beta1.Listener{
		Name:     "invalid-listener",
		Hostname: nil,
		Port:     443,
		Protocol: v1beta1.HTTPSProtocolType,
		TLS:      nil, // missing TLS config
	}

	// nolint:gosec
	secretPath := "/etc/nginx/secrets/secret"

	tests := []struct {
		graph    *graph
		expected Configuration
		msg      string
	}{
		{
			graph: &graph{
				GatewayClass: &gatewayClass{
					Source: &v1beta1.GatewayClass{},
					Valid:  true,
				},
				Gateway: &gateway{
					Source:    &v1beta1.Gateway{},
					Listeners: map[string]*listener{},
				},
				Routes: map[types.NamespacedName]*route{},
			},
			expected: Configuration{
				HTTPServers: []VirtualServer{},
				SSLServers:  []VirtualServer{},
				Upstreams:   []Upstream{},
			},
			msg: "no listeners and routes",
		},
		{
			graph: &graph{
				GatewayClass: &gatewayClass{
					Source: &v1beta1.GatewayClass{},
					Valid:  true,
				},
				Gateway: &gateway{
					Source: &v1beta1.Gateway{},
					Listeners: map[string]*listener{
						"listener-80-1": {
							Source:            listener80,
							Valid:             true,
							Routes:            map[types.NamespacedName]*route{},
							AcceptedHostnames: map[string]struct{}{},
						},
					},
				},
				Routes: map[types.NamespacedName]*route{},
			},
			expected: Configuration{
				HTTPServers: []VirtualServer{},
				SSLServers:  []VirtualServer{},
				Upstreams:   []Upstream{},
			},
			msg: "http listener with no routes",
		},
		{
			graph: &graph{
				GatewayClass: &gatewayClass{
					Source: &v1beta1.GatewayClass{},
					Valid:  true,
				},
				Gateway: &gateway{
					Source: &v1beta1.Gateway{},
					Listeners: map[string]*listener{
						"listener-443-1": {
							Source:            listener443, // nil hostname
							Valid:             true,
							Routes:            map[types.NamespacedName]*route{},
							AcceptedHostnames: map[string]struct{}{},
							SecretPath:        secretPath,
						},
						"listener-443-with-hostname": {
							Source:            listener443WithHostname, // non-nil hostname
							Valid:             true,
							Routes:            map[types.NamespacedName]*route{},
							AcceptedHostnames: map[string]struct{}{},
							SecretPath:        secretPath,
						},
					},
				},
				Routes: map[types.NamespacedName]*route{},
			},
			expected: Configuration{
				HTTPServers: []VirtualServer{},
				SSLServers: []VirtualServer{
					{
						Hostname: string(hostname),
						SSL:      &SSL{CertificatePath: secretPath},
					},
					{
						Hostname: wildcardHostname,
						SSL:      &SSL{CertificatePath: secretPath},
					},
				},
				Upstreams: []Upstream{},
			},
			msg: "https listeners with no routes",
		},
		{
			graph: &graph{
				GatewayClass: &gatewayClass{
					Source: &v1beta1.GatewayClass{},
					Valid:  true,
				},
				Gateway: &gateway{
					Source: &v1beta1.Gateway{},
					Listeners: map[string]*listener{
						"invalid-listener": {
							Source: invalidListener,
							Valid:  false,
							Routes: map[types.NamespacedName]*route{
								{Namespace: "test", Name: "https-hr-1"}: httpsRouteHR1,
								{Namespace: "test", Name: "https-hr-2"}: httpsRouteHR2,
							},
							AcceptedHostnames: map[string]struct{}{
								"foo.example.com": {},
								"bar.example.com": {},
							},
							SecretPath: "",
						},
					},
				},
				Routes: map[types.NamespacedName]*route{
					{Namespace: "test", Name: "https-hr-1"}: httpsRouteHR1,
					{Namespace: "test", Name: "https-hr-2"}: httpsRouteHR2,
				},
			},
			expected: Configuration{
				HTTPServers: []VirtualServer{},
				SSLServers:  []VirtualServer{},
				Upstreams:   []Upstream{},
			},
			msg: "invalid listener",
		},
		{
			graph: &graph{
				GatewayClass: &gatewayClass{
					Source: &v1beta1.GatewayClass{},
					Valid:  true,
				},
				Gateway: &gateway{
					Source: &v1beta1.Gateway{},
					Listeners: map[string]*listener{
						"listener-80-1": {
							Source: listener80,
							Valid:  true,
							Routes: map[types.NamespacedName]*route{
								{Namespace: "test", Name: "hr-1"}: routeHR1,
								{Namespace: "test", Name: "hr-2"}: routeHR2,
							},
							AcceptedHostnames: map[string]struct{}{
								"foo.example.com": {},
								"bar.example.com": {},
							},
						},
					},
				},
				Routes: map[types.NamespacedName]*route{
					{Namespace: "test", Name: "hr-1"}: routeHR1,
					{Namespace: "test", Name: "hr-2"}: routeHR2,
				},
				Backends: map[backendService]backend{
					fooBackendSvc: fooBackend,
				},
			},
			expected: Configuration{
				HTTPServers: []VirtualServer{
					{
						Hostname: "bar.example.com",
						PathRules: []PathRule{
							{
								Path: "/",
								MatchRules: []MatchRule{
									{
										MatchIdx:     0,
										RuleIdx:      0,
										UpstreamName: fooUpstreamName,
										Source:       hr2,
									},
								},
							},
						},
					},
					{
						Hostname: "foo.example.com",
						PathRules: []PathRule{
							{
								Path: "/",
								MatchRules: []MatchRule{
									{
										MatchIdx:     0,
										RuleIdx:      0,
										UpstreamName: fooUpstreamName,
										Source:       hr1,
									},
								},
							},
						},
					},
				},
				SSLServers: []VirtualServer{},
				Upstreams:  []Upstream{fooUpstream},
			},
			msg: "one http listener with two routes for different hostnames",
		},
		{
			graph: &graph{
				GatewayClass: &gatewayClass{
					Source: &v1beta1.GatewayClass{},
					Valid:  true,
				},
				Gateway: &gateway{
					Source: &v1beta1.Gateway{},
					Listeners: map[string]*listener{
						"listener-443-1": {
							Source:     listener443,
							Valid:      true,
							SecretPath: secretPath,
							Routes: map[types.NamespacedName]*route{
								{Namespace: "test", Name: "https-hr-1"}: httpsRouteHR1,
								{Namespace: "test", Name: "https-hr-2"}: httpsRouteHR2,
							},
							AcceptedHostnames: map[string]struct{}{
								"foo.example.com": {},
								"bar.example.com": {},
							},
						},
						"listener-443-with-hostname": {
							Source:     listener443WithHostname,
							Valid:      true,
							SecretPath: secretPath,
							Routes: map[types.NamespacedName]*route{
								{Namespace: "test", Name: "https-hr-5"}: httpsRouteHR5,
							},
							AcceptedHostnames: map[string]struct{}{
								"example.com": {},
							},
						},
					},
				},
				Routes: map[types.NamespacedName]*route{
					{Namespace: "test", Name: "https-hr-1"}: httpsRouteHR1,
					{Namespace: "test", Name: "https-hr-2"}: httpsRouteHR2,
					{Namespace: "test", Name: "https-hr-5"}: httpsRouteHR5,
				},
				Backends: map[backendService]backend{
					fooBackendSvc: fooBackend,
				},
			},
			expected: Configuration{
				HTTPServers: []VirtualServer{},
				SSLServers: []VirtualServer{
					{
						Hostname: "bar.example.com",
						PathRules: []PathRule{
							{
								Path: "/",
								MatchRules: []MatchRule{
									{
										MatchIdx:     0,
										RuleIdx:      0,
										UpstreamName: fooUpstreamName,
										Source:       httpsHR2,
									},
								},
							},
						},
						SSL: &SSL{
							CertificatePath: secretPath,
						},
					},
					{
						Hostname: "example.com",
						PathRules: []PathRule{
							{
								Path: "/",
								MatchRules: []MatchRule{
									{
										MatchIdx:     0,
										RuleIdx:      0,
										UpstreamName: InvalidBackendRef,
										Source:       httpsHR5,
									},
								},
							},
						},
						SSL: &SSL{
							CertificatePath: secretPath,
						},
					},
					{
						Hostname: "foo.example.com",
						PathRules: []PathRule{
							{
								Path: "/",
								MatchRules: []MatchRule{
									{
										MatchIdx:     0,
										RuleIdx:      0,
										UpstreamName: fooUpstreamName,
										Source:       httpsHR1,
									},
								},
							},
						},
						SSL: &SSL{
							CertificatePath: secretPath,
						},
					},
					{
						Hostname: wildcardHostname,
						SSL:      &SSL{CertificatePath: secretPath},
					},
				},
				Upstreams: []Upstream{fooUpstream},
			},
			msg: "two https listeners each with routes for different hostnames",
		},
		{
			graph: &graph{
				GatewayClass: &gatewayClass{
					Source: &v1beta1.GatewayClass{},
					Valid:  true,
				},
				Gateway: &gateway{
					Source: &v1beta1.Gateway{},
					Listeners: map[string]*listener{
						"listener-80-1": {
							Source: listener80,
							Valid:  true,
							Routes: map[types.NamespacedName]*route{
								{Namespace: "test", Name: "hr-3"}: routeHR3,
								{Namespace: "test", Name: "hr-4"}: routeHR4,
							},
							AcceptedHostnames: map[string]struct{}{
								"foo.example.com": {},
							},
						},
						"listener-443-1": {
							Source:     listener443,
							Valid:      true,
							SecretPath: secretPath,
							Routes: map[types.NamespacedName]*route{
								{Namespace: "test", Name: "https-hr-3"}: httpsRouteHR3,
								{Namespace: "test", Name: "https-hr-4"}: httpsRouteHR4,
							},
							AcceptedHostnames: map[string]struct{}{
								"foo.example.com": {},
							},
						},
					},
				},
				Routes: map[types.NamespacedName]*route{
					{Namespace: "test", Name: "hr-3"}:       routeHR3,
					{Namespace: "test", Name: "hr-4"}:       routeHR4,
					{Namespace: "test", Name: "https-hr-3"}: httpsRouteHR3,
					{Namespace: "test", Name: "https-hr-4"}: httpsRouteHR4,
				},
				Backends: map[backendService]backend{
					fooBackendSvc: fooBackend,
				},
			},
			expected: Configuration{
				HTTPServers: []VirtualServer{
					{
						Hostname: "foo.example.com",
						PathRules: []PathRule{
							{
								Path: "/",
								MatchRules: []MatchRule{
									{
										MatchIdx:     0,
										RuleIdx:      0,
										UpstreamName: fooUpstreamName,
										Source:       hr3,
									},
									{
										MatchIdx:     0,
										RuleIdx:      1,
										UpstreamName: fooUpstreamName,
										Source:       hr4,
									},
								},
							},
							{
								Path: "/fourth",
								MatchRules: []MatchRule{
									{
										MatchIdx:     0,
										RuleIdx:      0,
										UpstreamName: fooUpstreamName,
										Source:       hr4,
									},
								},
							},
							{
								Path: "/third",
								MatchRules: []MatchRule{
									{
										MatchIdx:     0,
										RuleIdx:      1,
										UpstreamName: fooUpstreamName,
										Source:       hr3,
									},
								},
							},
						},
					},
				},
				SSLServers: []VirtualServer{
					{
						Hostname: "foo.example.com",
						SSL: &SSL{
							CertificatePath: secretPath,
						},
						PathRules: []PathRule{
							{
								Path: "/",
								MatchRules: []MatchRule{
									{
										MatchIdx:     0,
										RuleIdx:      0,
										UpstreamName: fooUpstreamName,
										Source:       httpsHR3,
									},
									{
										MatchIdx:     0,
										RuleIdx:      1,
										UpstreamName: fooUpstreamName,
										Source:       httpsHR4,
									},
								},
							},
							{
								Path: "/fourth",
								MatchRules: []MatchRule{
									{
										MatchIdx:     0,
										RuleIdx:      0,
										UpstreamName: fooUpstreamName,
										Source:       httpsHR4,
									},
								},
							},
							{
								Path: "/third",
								MatchRules: []MatchRule{
									{
										MatchIdx:     0,
										RuleIdx:      1,
										UpstreamName: fooUpstreamName,
										Source:       httpsHR3,
									},
								},
							},
						},
					},
					{
						Hostname: wildcardHostname,
						SSL:      &SSL{CertificatePath: secretPath},
					},
				},
				Upstreams: []Upstream{fooUpstream},
			},
			msg: "one http and one https listener with two routes with the same hostname with and without collisions",
		},
		{
			graph: &graph{
				GatewayClass: &gatewayClass{
					Source:   &v1beta1.GatewayClass{},
					Valid:    false,
					ErrorMsg: "error",
				},
				Gateway: &gateway{
					Source: &v1beta1.Gateway{},
					Listeners: map[string]*listener{
						"listener-80-1": {
							Source: listener80,
							Valid:  true,
							Routes: map[types.NamespacedName]*route{
								{Namespace: "test", Name: "hr-1"}: routeHR1,
							},
							AcceptedHostnames: map[string]struct{}{
								"foo.example.com": {},
							},
						},
					},
				},
				Routes: map[types.NamespacedName]*route{
					{Namespace: "test", Name: "hr-1"}: routeHR1,
				},
			},
			expected: Configuration{},
			msg:      "invalid gatewayclass",
		},
		{
			graph: &graph{
				GatewayClass: nil,
				Gateway: &gateway{
					Source: &v1beta1.Gateway{},
					Listeners: map[string]*listener{
						"listener-80-1": {
							Source: listener80,
							Valid:  true,
							Routes: map[types.NamespacedName]*route{
								{Namespace: "test", Name: "hr-1"}: routeHR1,
							},
							AcceptedHostnames: map[string]struct{}{
								"foo.example.com": {},
							},
						},
					},
				},
				Routes: map[types.NamespacedName]*route{
					{Namespace: "test", Name: "hr-1"}: routeHR1,
				},
				Backends: map[backendService]backend{
					fooBackendSvc: fooBackend,
				},
			},
			expected: Configuration{},
			msg:      "missing gatewayclass",
		},
		{
			graph: &graph{
				GatewayClass: &gatewayClass{
					Source: &v1beta1.GatewayClass{},
					Valid:  true,
				},
				Gateway: nil,
				Routes:  map[types.NamespacedName]*route{},
			},
			expected: Configuration{},
			msg:      "missing gateway",
		},
	}

	for _, test := range tests {
		result := buildConfiguration(test.graph)
		if diff := cmp.Diff(test.expected, result); diff != "" {
			t.Errorf("buildConfiguration() %q mismatch (-want +got):\n%s", test.msg, diff)
		}
	}
}

func TestGetPath(t *testing.T) {
	tests := []struct {
		path     *v1beta1.HTTPPathMatch
		expected string
		msg      string
	}{
		{
			path:     &v1beta1.HTTPPathMatch{Value: helpers.GetStringPointer("/abc")},
			expected: "/abc",
			msg:      "normal case",
		},
		{
			path:     nil,
			expected: "/",
			msg:      "nil path",
		},
		{
			path:     &v1beta1.HTTPPathMatch{Value: nil},
			expected: "/",
			msg:      "nil value",
		},
		{
			path:     &v1beta1.HTTPPathMatch{Value: helpers.GetStringPointer("")},
			expected: "/",
			msg:      "empty value",
		},
	}

	for _, test := range tests {
		result := getPath(test.path)
		if result != test.expected {
			t.Errorf("getPath() returned %q but expected %q for the case of %q", result, test.expected, test.msg)
		}
	}
}

func TestMatchRuleGetMatch(t *testing.T) {
	hr := &v1beta1.HTTPRoute{
		Spec: v1beta1.HTTPRouteSpec{
			Rules: []v1beta1.HTTPRouteRule{
				{
					Matches: []v1beta1.HTTPRouteMatch{
						{
							Path: &v1beta1.HTTPPathMatch{
								Value: helpers.GetStringPointer("/path-1"),
							},
						},
						{
							Path: &v1beta1.HTTPPathMatch{
								Value: helpers.GetStringPointer("/path-2"),
							},
						},
					},
				},
				{
					Matches: []v1beta1.HTTPRouteMatch{
						{
							Path: &v1beta1.HTTPPathMatch{
								Value: helpers.GetStringPointer("/path-3"),
							},
						},
						{
							Path: &v1beta1.HTTPPathMatch{
								Value: helpers.GetStringPointer("/path-4"),
							},
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name,
		expPath string
		rule MatchRule
	}{
		{
			name:    "first match in first rule",
			expPath: "/path-1",
			rule:    MatchRule{MatchIdx: 0, RuleIdx: 0, Source: hr},
		},
		{
			name:    "second match in first rule",
			expPath: "/path-2",
			rule:    MatchRule{MatchIdx: 1, RuleIdx: 0, Source: hr},
		},
		{
			name:    "second match in second rule",
			expPath: "/path-4",
			rule:    MatchRule{MatchIdx: 1, RuleIdx: 1, Source: hr},
		},
	}

	for _, tc := range tests {
		actual := tc.rule.GetMatch()
		if *actual.Path.Value != tc.expPath {
			t.Errorf("MatchRule.GetMatch() returned incorrect match with path: %s, expected path: %s for test case: %q", *actual.Path.Value, tc.expPath, tc.name)
		}
	}
}

func TestGetListenerHostname(t *testing.T) {
	var emptyHostname v1beta1.Hostname
	var hostname v1beta1.Hostname = "example.com"

	tests := []struct {
		hostname *v1beta1.Hostname
		expected string
		msg      string
	}{
		{
			hostname: nil,
			expected: wildcardHostname,
			msg:      "nil hostname",
		},
		{
			hostname: &emptyHostname,
			expected: wildcardHostname,
			msg:      "empty hostname",
		},
		{
			hostname: &hostname,
			expected: string(hostname),
			msg:      "normal hostname",
		},
	}

	for _, test := range tests {
		result := getListenerHostname(test.hostname)
		if result != test.expected {
			t.Errorf("getListenerHostname() returned %q but expected %q for the case of %q", result, test.expected, test.msg)
		}
	}
}

func TestBuildUpstreams(t *testing.T) {

	fooEndpoints := []Endpoint{
		{
			Address: "10.0.0.0",
			Port:    8080,
		},
		{
			Address: "10.0.0.1",
			Port:    8080,
		},
		{
			Address: "10.0.0.2",
			Port:    8080,
		},
	}

	barEndpoints := []Endpoint{
		{
			Address: "11.0.0.0",
			Port:    80,
		},
		{
			Address: "11.0.0.1",
			Port:    80,
		},
		{
			Address: "11.0.0.2",
			Port:    80,
		},
		{
			Address: "11.0.0.3",
			Port:    80,
		},
	}

	backends := map[backendService]backend{
		backendService{name: "foo", namespace: "test", port: 80}:              {Endpoints: fooEndpoints},
		backendService{name: "bar", namespace: "test", port: 8080}:            {Endpoints: barEndpoints},
		backendService{name: "nil-endpoints", namespace: "test", port: 443}:   {Endpoints: nil},
		backendService{name: "empty-endpoints", namespace: "test", port: 443}: {Endpoints: []Endpoint{}},
	}

	expUpstreams := []Upstream{
		{Name: "test_foo_80", Endpoints: fooEndpoints},
		{Name: "test_bar_8080", Endpoints: barEndpoints},
		{Name: "test_nil-endpoints_443", Endpoints: nil},
		{Name: "test_empty-endpoints_443", Endpoints: nil},
	}

	upstreams := buildUpstreams(backends)

	if diff := helpers.Diff(expUpstreams, upstreams); diff != "" {
		t.Errorf("buildUpstreams() returned incorrect Upstreams, diff: %+v", diff)
	}
}

func TestGenerateUpstreamName(t *testing.T) {
	// empty backend service
	if name := generateUpstreamName(backendService{}); name != InvalidBackendRef {
		t.Errorf("generateUpstreamName() returned unexepected name: %s, expected: %s", name, InvalidBackendRef)
	}

	expName := "test_foo_9090"
	if name := generateUpstreamName(backendService{name: "foo", namespace: "test", port: 9090}); name != expName {
		t.Errorf("generateUpstreamName() returned unexepected name: %s, expected: %s", name, expName)
	}
}
