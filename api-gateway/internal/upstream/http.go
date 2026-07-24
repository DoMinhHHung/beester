package upstream

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/DoMinhHHung/beester/api-gateway/internal/identity"
	"github.com/DoMinhHHung/beester/api-gateway/internal/requestid"
)

type HTTPSpec struct {
	Name   string
	Target string
}

type HTTPRegistry struct {
	handlers  map[string]http.Handler
	names     []string
	transport *http.Transport
}

func NewHTTP(
	specs []HTTPSpec,
	logger *slog.Logger,
	userIDHeader string,
	forwardAuthorization bool,
) (*HTTPRegistry, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = http.ProxyFromEnvironment
	transport.DialContext = (&net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext
	transport.ForceAttemptHTTP2 = true
	transport.MaxIdleConns = 512
	transport.MaxIdleConnsPerHost = 128
	transport.IdleConnTimeout = 90 * time.Second
	transport.TLSHandshakeTimeout = 10 * time.Second
	transport.ExpectContinueTimeout = time.Second

	registry := &HTTPRegistry{
		handlers:  make(map[string]http.Handler, len(specs)),
		names:     make([]string, 0, len(specs)),
		transport: transport,
	}

	for _, spec := range specs {
		name := strings.TrimSpace(spec.Name)
		if name == "" {
			return nil, errors.New("HTTP upstream name is required")
		}
		if _, exists := registry.handlers[name]; exists {
			return nil, fmt.Errorf("duplicate HTTP upstream %q", name)
		}

		target, err := parseHTTPUpstreamTarget(spec.Target)
		if err != nil {
			return nil, fmt.Errorf("parse HTTP upstream %q: %w", name, err)
		}

		proxy := &httputil.ReverseProxy{
			Transport: transport,
			Rewrite: func(proxyRequest *httputil.ProxyRequest) {
				proxyRequest.SetURL(target)
				proxyRequest.Out.Host = target.Host
				proxyRequest.SetXForwarded()

				proxyRequest.Out.Header.Del(requestid.Header)
				if id := requestid.FromContext(proxyRequest.In.Context()); id != "" {
					proxyRequest.Out.Header.Set(requestid.Header, id)
				}

				if userIDHeader != "" {
					proxyRequest.Out.Header.Del(userIDHeader)
					if value, ok := identity.FromContext(proxyRequest.In.Context()); ok && value.UserID != "" {
						proxyRequest.Out.Header.Set(userIDHeader, value.UserID)
					}
				}

				if !forwardAuthorization {
					proxyRequest.Out.Header.Del("Authorization")
				}
			},
			ModifyResponse: func(response *http.Response) error {
				response.Header.Del("Server")
				if response.Request != nil {
					if id := requestid.FromContext(response.Request.Context()); id != "" {
						response.Header.Set(requestid.Header, id)
					}
				}
				return nil
			},
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, proxyErr error) {
				logger.ErrorContext(
					r.Context(),
					"HTTP upstream proxy failed",
					slog.String("request_id", requestid.FromContext(r.Context())),
					slog.String("upstream", name),
					slog.Any("error", proxyErr),
				)
				if errors.Is(proxyErr, context.DeadlineExceeded) || errors.Is(r.Context().Err(), context.DeadlineExceeded) {
					http.Error(w, "gateway timeout", http.StatusGatewayTimeout)
					return
				}
				http.Error(w, "bad gateway", http.StatusBadGateway)
			},
		}

		registry.handlers[name] = proxy
		registry.names = append(registry.names, name)
	}

	return registry, nil
}

func parseHTTPUpstreamTarget(value string) (*url.URL, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, errors.New("target is required")
	}
	target, err := url.Parse(value)
	if err != nil {
		return nil, err
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme %q", target.Scheme)
	}
	if target.Host == "" {
		return nil, errors.New("target host is required")
	}
	return target, nil
}

func (r *HTTPRegistry) Handler(name string) (http.Handler, bool) {
	if r == nil || r.handlers == nil {
		return nil, false
	}
	handler, ok := r.handlers[name]
	return handler, ok
}

func (r *HTTPRegistry) Names() []string {
	if r == nil {
		return nil
	}
	return append([]string(nil), r.names...)
}

func (r *HTTPRegistry) CloseIdleConnections() {
	if r != nil && r.transport != nil {
		r.transport.CloseIdleConnections()
	}
}
