package proxy

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/DoMinhHHung/beester/api-gateway/internal/requestid"
	"github.com/DoMinhHHung/beester/api-gateway/internal/routing"
	"github.com/DoMinhHHung/beester/api-gateway/internal/upstream"
)

type HTTPDispatcher struct {
	routes         *routing.Table
	upstreams      *upstream.HTTPRegistry
	logger         *slog.Logger
	requestTimeout time.Duration
}

func NewHTTPDispatcher(
	routes *routing.Table,
	upstreams *upstream.HTTPRegistry,
	logger *slog.Logger,
	requestTimeout time.Duration,
) *HTTPDispatcher {
	return &HTTPDispatcher{
		routes:         routes,
		upstreams:      upstreams,
		logger:         logger,
		requestTimeout: requestTimeout,
	}
}

func (d *HTTPDispatcher) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if d == nil || d.routes == nil || d.upstreams == nil {
		http.Error(w, "gateway unavailable", http.StatusServiceUnavailable)
		return
	}

	route, ok := d.routes.Match(r)
	if !ok {
		http.NotFound(w, r)
		return
	}

	handler, ok := d.upstreams.Handler(route.Upstream)
	if !ok {
		d.logger.ErrorContext(
			r.Context(),
			"configured HTTP upstream is unavailable",
			slog.String("request_id", requestid.FromContext(r.Context())),
			slog.String("upstream", route.Upstream),
		)
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return
	}

	if d.requestTimeout <= 0 {
		handler.ServeHTTP(w, r)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), d.requestTimeout)
	defer cancel()
	handler.ServeHTTP(w, r.WithContext(ctx))
}
