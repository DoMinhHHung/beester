package routing

import (
	"fmt"
	"net/http"
	"strings"
)

type Route struct {
	Method   string
	Pattern  string
	Upstream string
}

type routeEntry struct {
	route Route
}

func (*routeEntry) ServeHTTP(http.ResponseWriter, *http.Request) {}

type Table struct {
	mux    *http.ServeMux
	routes []Route
}

func New(routes []Route) (*Table, error) {
	mux := http.NewServeMux()
	copiedRoutes := make([]Route, 0, len(routes))

	for _, route := range routes {
		normalized := Route{
			Method:   strings.ToUpper(strings.TrimSpace(route.Method)),
			Pattern:  strings.TrimSpace(route.Pattern),
			Upstream: strings.TrimSpace(route.Upstream),
		}
		if normalized.Method == "" {
			return nil, fmt.Errorf("route HTTP method is required")
		}
		if normalized.Pattern == "" || !strings.HasPrefix(normalized.Pattern, "/") {
			return nil, fmt.Errorf("route pattern %q must start with /", normalized.Pattern)
		}
		if normalized.Upstream == "" {
			return nil, fmt.Errorf("route upstream is required")
		}

		pattern := normalized.Method + " " + normalized.Pattern
		if err := registerRoute(mux, pattern, &routeEntry{route: normalized}); err != nil {
			return nil, err
		}
		copiedRoutes = append(copiedRoutes, normalized)
	}

	return &Table{mux: mux, routes: copiedRoutes}, nil
}

func (t *Table) Match(r *http.Request) (Route, bool) {
	if t == nil || t.mux == nil || r == nil {
		return Route{}, false
	}
	handler, pattern := t.mux.Handler(r)
	if pattern == "" {
		return Route{}, false
	}
	entry, ok := handler.(*routeEntry)
	if !ok {
		return Route{}, false
	}
	return entry.route, true
}

func (t *Table) Len() int {
	if t == nil {
		return 0
	}
	return len(t.routes)
}

func registerRoute(mux *http.ServeMux, pattern string, handler http.Handler) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("register route pattern %q: %v", pattern, recovered)
		}
	}()
	mux.Handle(pattern, handler)
	return nil
}
