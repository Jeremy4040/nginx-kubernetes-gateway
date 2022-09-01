package config

import (
	"bytes"
	"fmt"
	"text/template"
)

// templateExecutor generates NGINX configuration using a template.
// Template parsing or executing errors can only occur if there is a bug in the template, so they are handled with panics.
// For now, we only generate configuration with NGINX http servers and upstreams, but in the future we will also need to generate
// the main NGINX configuration file and stream servers.
type templateExecutor struct {
	httpServersTemplate   *template.Template
	httpUpstreamsTemplate *template.Template
}

func newTemplateExecutor() *templateExecutor {
	httpT, err := template.New("server").Parse(httpServersTemplate)
	if err != nil {
		panic(fmt.Errorf("failed to parse http servers template: %w", err))
	}

	upstreamT, err := template.New("server").Parse(httpUpstreamsTemplate)
	if err != nil {
		panic(fmt.Errorf("failed to parse upstream template: %w", err))
	}

	return &templateExecutor{httpServersTemplate: httpT, httpUpstreamsTemplate: upstreamT}
}

func (e *templateExecutor) ExecuteForHTTP(servers httpServers) []byte {
	var buf bytes.Buffer

	err := e.httpServersTemplate.Execute(&buf, servers)
	if err != nil {
		panic(fmt.Errorf("failed to execute http servers template: %w", err))
	}

	return buf.Bytes()
}

func (e *templateExecutor) ExecuteForUpstreams(upstreams httpUpstreams) []byte {
	var buf bytes.Buffer

	err := e.httpUpstreamsTemplate.Execute(&buf, upstreams)
	if err != nil {
		panic(fmt.Errorf("failed to execute http upstream template: %w", err))
	}

	return buf.Bytes()
}
