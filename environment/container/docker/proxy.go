package docker

import (
	"os"
	"strings"
)

type proxyVars struct {
	http  string
	https string
	no    string
}

func (p proxyVars) empty() bool {
	return p.http == "" && p.https == ""
}

type proxyVarKey string

var (
	httpProxy  proxyVarKey = "http_proxy"
	httpsProxy proxyVarKey = "https_proxy"
	noProxy    proxyVarKey = "no_proxy"
)

// keys return both the lower case and upper case env var keys.
// e.g. http_proxy and HTTP_PROXY
func (p proxyVarKey) Keys() []string {
	return []string{string(p), strings.ToUpper(string(p))}
}

func (d dockerRuntime) proxyEnvVars(env map[string]string) proxyVars {
	getVal := func(key proxyVarKey) string {
		for _, k := range key.Keys() {
			// config
			if val, ok := env[k]; ok {
				return val
			}
			// os
			if val := os.Getenv(k); val != "" {
				return val
			}
		}
		return ""
	}

	return proxyVars{
		http:  getVal(httpProxy),
		https: getVal(httpsProxy),
		no:    getVal(noProxy),
	}
}
