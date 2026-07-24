package grpcrouting

import "testing"

func TestTableUsesLongestPrefix(t *testing.T) {
	table, err := New([]Route{
		{Prefix: "/example.v1/", Upstream: "default"},
		{Prefix: "/example.v1.UserService/", Upstream: "users"},
	})
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	route, ok := table.Match("/example.v1.UserService/Get")
	if !ok || route.Upstream != "users" {
		t.Fatalf("unexpected match: %#v %v", route, ok)
	}
}

func TestTableReturnsNoMatch(t *testing.T) {
	table, err := New(nil)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, ok := table.Match("/missing.Service/Get"); ok {
		t.Fatal("expected no match")
	}
}
