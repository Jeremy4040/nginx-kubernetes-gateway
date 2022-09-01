package state

import (
	"fmt"
	"sort"

	"sigs.k8s.io/gateway-api/apis/v1beta1"
)

const wildcardHostname = "~^"

// Configuration is an internal representation of Gateway configuration.
// We can think of Configuration as an intermediate state between the Gateway API resources and the data plane (NGINX)
// configuration.
type Configuration struct {
	// HTTPServers holds all HTTPServers.
	// FIXME(pleshakov) We assume that all servers are HTTP and listen on port 80.
	HTTPServers []VirtualServer
	// SSLServers holds all SSLServers.
	// FIXME(kate-osborn) We assume that all SSL servers listen on port 443.
	SSLServers []VirtualServer
	// Upstreams holds all Upstreams.
	Upstreams []Upstream
}

// VirtualServer is a virtual server.
type VirtualServer struct {
	// Hostname is the hostname of the server.
	Hostname string
	// PathRules is a collection of routing rules.
	PathRules []PathRule
	// SSL holds the SSL configuration options fo the server.
	SSL *SSL
}

type Upstream struct {
	// Name is the name of the Upstream. Will be unique for each service/port combination.
	Name string
	// Endpoints are the endpoints of the Upstream.
	Endpoints []Endpoint
}

type SSL struct {
	// CertificatePath is the path to the certificate file.
	CertificatePath string
}

// PathRule represents routing rules that share a common path.
type PathRule struct {
	// Path is a path. For example, '/hello'.
	Path string
	// MatchRules holds routing rules.
	MatchRules []MatchRule
}

// MatchRule represents a routing rule. It corresponds directly to a Match in the HTTPRoute resource.
// An HTTPRoute is guaranteed to have at least one rule with one match.
// If no rule or match is specified by the user, the default rule {{path:{ type: "PathPrefix", value: "/"}}} is set by the schema.
type MatchRule struct {
	// MatchIdx is the index of the rule in the Rule.Matches.
	MatchIdx int
	// RuleIdx is the index of the corresponding rule in the HTTPRoute.
	RuleIdx int
	// UpstreamName is the name of the upstream for this routing rule.
	UpstreamName string
	// Source is the corresponding HTTPRoute resource.
	Source *v1beta1.HTTPRoute
}

// GetMatch returns the HTTPRouteMatch of the Route .
func (r *MatchRule) GetMatch() v1beta1.HTTPRouteMatch {
	return r.Source.Spec.Rules[r.RuleIdx].Matches[r.MatchIdx]
}

// buildConfiguration builds the Configuration from the graph.
// FIXME(pleshakov) For now we only handle paths with prefix matches. Handle exact and regex matches
func buildConfiguration(graph *graph) Configuration {
	if graph.GatewayClass == nil || !graph.GatewayClass.Valid {
		return Configuration{}
	}

	if graph.Gateway == nil {
		return Configuration{}
	}

	upstreams := buildUpstreams(graph.Backends)

	builderFactory := newVirtualServerBuilderFactory()

	for _, l := range graph.Gateway.Listeners {
		// only upsert listeners that are valid
		if l.Valid {
			builder := builderFactory.getBuilderForProtocol(l.Source.Protocol)
			builder.upsertListener(l)
		}
	}

	config := Configuration{
		HTTPServers: builderFactory.getBuilderForProtocol(v1beta1.HTTPProtocolType).build(),
		SSLServers:  builderFactory.getBuilderForProtocol(v1beta1.HTTPSProtocolType).build(),
		Upstreams:   upstreams,
	}

	return config
}

type virtualServerBuilderFactory struct {
	http *virtualServerBuilder
	ssl  *virtualServerBuilder
}

func newVirtualServerBuilderFactory() *virtualServerBuilderFactory {
	return &virtualServerBuilderFactory{
		http: newVirtualServerBuilder(v1beta1.HTTPProtocolType),
		ssl:  newVirtualServerBuilder(v1beta1.HTTPSProtocolType),
	}
}

func (b *virtualServerBuilderFactory) getBuilderForProtocol(protocol v1beta1.ProtocolType) *virtualServerBuilder {
	switch protocol {
	case v1beta1.HTTPProtocolType:
		return b.http
	case v1beta1.HTTPSProtocolType:
		return b.ssl
	default:
		panic(fmt.Sprintf("protocol %s not supported", protocol))
	}
}

type virtualServerBuilder struct {
	protocolType     v1beta1.ProtocolType
	rulesPerHost     map[string]map[string]PathRule
	listenersForHost map[string]*listener
	listeners        []*listener
}

func newVirtualServerBuilder(protocolType v1beta1.ProtocolType) *virtualServerBuilder {
	return &virtualServerBuilder{
		protocolType:     protocolType,
		rulesPerHost:     make(map[string]map[string]PathRule),
		listenersForHost: make(map[string]*listener),
		listeners:        make([]*listener, 0),
	}
}

func (b *virtualServerBuilder) upsertListener(l *listener) {
	if b.protocolType == v1beta1.HTTPSProtocolType {
		b.listeners = append(b.listeners, l)
	}

	for _, r := range l.Routes {
		var hostnames []string

		for _, h := range r.Source.Spec.Hostnames {
			if _, exist := l.AcceptedHostnames[string(h)]; exist {
				hostnames = append(hostnames, string(h))
			}
		}

		for _, h := range hostnames {
			b.listenersForHost[h] = l

			if _, exist := b.rulesPerHost[h]; !exist {
				b.rulesPerHost[h] = make(map[string]PathRule)
			}
		}

		for i, rule := range r.Source.Spec.Rules {
			for _, h := range hostnames {
				for j, m := range rule.Matches {
					path := getPath(m.Path)

					rule, exist := b.rulesPerHost[h][path]
					if !exist {
						rule.Path = path
					}

					rule.MatchRules = append(rule.MatchRules, MatchRule{
						MatchIdx:     j,
						RuleIdx:      i,
						UpstreamName: generateUpstreamName(r.BackendServices[ruleIndex(i)]),
						Source:       r.Source,
					})

					b.rulesPerHost[h][path] = rule
				}
			}
		}
	}
}

func (b *virtualServerBuilder) build() []VirtualServer {
	servers := make([]VirtualServer, 0, len(b.rulesPerHost)+len(b.listeners))

	for h, rules := range b.rulesPerHost {
		s := VirtualServer{
			Hostname:  h,
			PathRules: make([]PathRule, 0, len(rules)),
		}

		l, ok := b.listenersForHost[h]
		if !ok {
			panic(fmt.Sprintf("no listener found for hostname: %s", h))
		}

		if l.SecretPath != "" {
			s.SSL = &SSL{CertificatePath: l.SecretPath}
		}

		for _, r := range rules {
			sortMatchRules(r.MatchRules)

			s.PathRules = append(s.PathRules, r)
		}

		// sort rules for predictable order
		sort.Slice(s.PathRules, func(i, j int) bool {
			return s.PathRules[i].Path < s.PathRules[j].Path
		})

		servers = append(servers, s)
	}

	for _, l := range b.listeners {
		hostname := getListenerHostname(l.Source.Hostname)
		// generate a 404 ssl server block for listeners with no routes or listeners with wildcard (match-all) routes
		// FIXME(kate-osborn): when we support regex hostnames (e.g. *.example.com) we will have to modify this check to catch regex hostnames.
		if len(l.Routes) == 0 || hostname == wildcardHostname {
			servers = append(servers, VirtualServer{
				Hostname: hostname,
				SSL:      &SSL{CertificatePath: l.SecretPath},
			})
		}
	}

	sort.Slice(servers, func(i, j int) bool {
		return servers[i].Hostname < servers[j].Hostname
	})

	return servers
}

func getListenerHostname(h *v1beta1.Hostname) string {
	name := getHostname(h)
	if name == "" {
		return wildcardHostname
	}

	return name
}

func getPath(path *v1beta1.HTTPPathMatch) string {
	if path == nil || path.Value == nil || *path.Value == "" {
		return "/"
	}
	return *path.Value
}
