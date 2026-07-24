package config

import "testing"

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				AppEnv:                "development",
				HTTPAddr:              ":8080",
				GRPCTransportSecurity: GRPCTransportSecurityTLS,
			},
		},
		{
			name: "missing app env",
			config: Config{
				HTTPAddr:              ":8080",
				GRPCTransportSecurity: GRPCTransportSecurityTLS,
			},
			wantErr: true,
		},
		{
			name: "missing http addr",
			config: Config{
				AppEnv:                "development",
				GRPCTransportSecurity: GRPCTransportSecurityTLS,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestParseGRPCUpstreams(t *testing.T) {
	upstreams, err := parseGRPCUpstreams("auth=dns:///auth:50051,user=dns:///user:50051")
	if err != nil {
		t.Fatalf("parse upstreams: %v", err)
	}
	if len(upstreams) != 2 {
		t.Fatalf("expected 2 upstreams, got %d", len(upstreams))
	}
	if upstreams[0].Name != "auth" || upstreams[0].Target != "dns:///auth:50051" {
		t.Fatalf("unexpected first upstream: %#v", upstreams[0])
	}
}

func TestParseGRPCUpstreamsRejectsDuplicates(t *testing.T) {
	if _, err := parseGRPCUpstreams("auth=localhost:1,auth=localhost:2"); err == nil {
		t.Fatal("expected duplicate upstream error")
	}
}

func TestParseHTTPRoutes(t *testing.T) {
	routes, err := parseHTTPRoutes("GET:/api/users/{id}=users,POST:/api/users=users")
	if err != nil {
		t.Fatalf("parse routes: %v", err)
	}
	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(routes))
	}
	if routes[0].Method != "GET" || routes[0].Pattern != "/api/users/{id}" || routes[0].Upstream != "users" {
		t.Fatalf("unexpected first route: %#v", routes[0])
	}
}

func TestParseHTTPRoutesRejectsDuplicates(t *testing.T) {
	if _, err := parseHTTPRoutes("GET:/users=a,GET:/users=b"); err == nil {
		t.Fatal("expected duplicate HTTP route error")
	}
}

func TestParseGRPCRoutes(t *testing.T) {
	routes, err := parseGRPCRoutes("/example.v1.UserService/=users,/example.v1.AuthService/=auth")
	if err != nil {
		t.Fatalf("parse gRPC routes: %v", err)
	}
	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(routes))
	}
}

func TestConfigValidateRejectsUnknownUpstreams(t *testing.T) {
	cfg := Config{
		AppEnv:                "development",
		HTTPAddr:              ":8080",
		GRPCTransportSecurity: GRPCTransportSecurityInsecure,
		HTTPRoutes:            []HTTPRoute{{Method: "GET", Pattern: "/users", Upstream: "missing"}},
	}
	if err := cfg.validate(); err == nil {
		t.Fatal("expected unknown HTTP upstream error")
	}

	cfg.HTTPRoutes = nil
	cfg.GRPCRoutes = []GRPCRoute{{Prefix: "/example.Service/", Upstream: "missing"}}
	cfg.GRPCAddr = ":9090"
	if err := cfg.validate(); err == nil {
		t.Fatal("expected unknown gRPC upstream error")
	}
}
