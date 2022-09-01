package config

var httpUpstreamsTemplate = `{{ range $u := .Upstreams }}
upstream {{ $u.Name }} {
	{{ range $server := $u.Servers }} 
	server {{ $server.Address }};
	{{ end }}
}
{{ end }}`
