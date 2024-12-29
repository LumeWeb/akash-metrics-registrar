package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

// Config holds proxy configuration
type Config struct {
	TargetHost    string
	TargetPort    int
	TargetPath    string
	FlushInterval time.Duration
}

// NewMetricsProxy creates a new reverse proxy for the metrics endpoint
func NewMetricsProxy(cfg Config) (*httputil.ReverseProxy, error) {
	target, err := url.Parse(fmt.Sprintf("http://%s:%d", cfg.TargetHost, cfg.TargetPort))
	if err != nil {
		return nil, err
	}

	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.Out.URL.Path = pr.In.URL.Path
			pr.SetXForwarded()
		},
		FlushInterval: cfg.FlushInterval,
	}

	return proxy, nil
}

// WithBasicAuth wraps a handler with basic auth using the metrics password
func WithBasicAuth(handler http.Handler, password string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, pass, ok := r.BasicAuth()
		if !ok || pass != password {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		handler.ServeHTTP(w, r)
	})
}
