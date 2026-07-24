package routing

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTableMatchesRoute(t *testing.T) {
	table, err := New([]Route{{Method: http.MethodGet, Pattern: "/api/users/{id}", Upstream: "users"}})
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/users/123", nil)
	route, ok := table.Match(req)
	if !ok || route.Upstream != "users" {
		t.Fatalf("unexpected match: %#v %v", route, ok)
	}
}

func TestTableDoesNotMatchWrongMethod(t *testing.T) {
	table, err := New([]Route{{Method: http.MethodGet, Pattern: "/users", Upstream: "users"}})
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, ok := table.Match(httptest.NewRequest(http.MethodPost, "/users", nil)); ok {
		t.Fatal("expected no match")
	}
}

func TestTableRejectsConflictingPatterns(t *testing.T) {
	_, err := New([]Route{
		{Method: http.MethodGet, Pattern: "/users/{id}", Upstream: "a"},
		{Method: http.MethodGet, Pattern: "/users/{name}", Upstream: "b"},
	})
	if err == nil {
		t.Fatal("expected conflicting pattern error")
	}
}
