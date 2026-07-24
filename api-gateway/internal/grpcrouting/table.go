package grpcrouting

import (
	"fmt"
	"sort"
	"strings"
)

type Route struct {
	Prefix   string
	Upstream string
}

type Table struct {
	routes []Route
}

func New(routes []Route) (*Table, error) {
	copied := make([]Route, 0, len(routes))
	prefixes := make(map[string]struct{}, len(routes))

	for _, route := range routes {
		normalized := Route{
			Prefix:   strings.TrimSpace(route.Prefix),
			Upstream: strings.TrimSpace(route.Upstream),
		}
		if normalized.Prefix == "" || !strings.HasPrefix(normalized.Prefix, "/") {
			return nil, fmt.Errorf("gRPC route prefix %q must start with /", normalized.Prefix)
		}
		if normalized.Upstream == "" {
			return nil, fmt.Errorf("gRPC route %q upstream is required", normalized.Prefix)
		}
		if _, exists := prefixes[normalized.Prefix]; exists {
			return nil, fmt.Errorf("duplicate gRPC route prefix %q", normalized.Prefix)
		}
		prefixes[normalized.Prefix] = struct{}{}
		copied = append(copied, normalized)
	}

	sort.SliceStable(copied, func(i, j int) bool {
		return len(copied[i].Prefix) > len(copied[j].Prefix)
	})
	return &Table{routes: copied}, nil
}

func (t *Table) Match(fullMethod string) (Route, bool) {
	if t == nil {
		return Route{}, false
	}
	for _, route := range t.routes {
		if strings.HasPrefix(fullMethod, route.Prefix) {
			return route, true
		}
	}
	return Route{}, false
}

func (t *Table) Len() int {
	if t == nil {
		return 0
	}
	return len(t.routes)
}
