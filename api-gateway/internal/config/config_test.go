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
			wantErr: false,
		},
		{
			name: "missing app env",
			config: Config{
				HTTPAddr: ":8080",
			},
			wantErr: true,
		},
		{
			name: "missing http addr",
			config: Config{
				AppEnv: "development",
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
	upstreams, err := parseGRPCUpstreams(
		"auth=dns:///auth:50051,user=dns:///user:50051",
	)
	if err != nil {
		t.Fatalf("parse upstreams: %v", err)
	}

	if len(upstreams) != 2 {
		t.Fatalf(
			"expected 2 upstreams, got %d",
			len(upstreams),
		)
	}

	if got, want := upstreams[0].Name, "auth"; got != want {
		t.Fatalf(
			"expected first upstream name %q, got %q",
			want,
			got,
		)
	}

	if got, want := upstreams[0].Target, "dns:///auth:50051"; got != want {
		t.Fatalf(
			"expected first upstream target %q, got %q",
			want,
			got,
		)
	}
}

func TestParseGRPCUpstreamsRejectsDuplicates(t *testing.T) {
	_, err := parseGRPCUpstreams(
		"auth=localhost:50051,auth=localhost:50052",
	)

	if err == nil {
		t.Fatal("expected duplicate upstream error")
	}
}

func TestParseGRPCUpstreamsAllowsEmptyValue(t *testing.T) {
	upstreams, err := parseGRPCUpstreams("")
	if err != nil {
		t.Fatalf("parse empty upstreams: %v", err)
	}

	if len(upstreams) != 0 {
		t.Fatalf(
			"expected no upstreams, got %d",
			len(upstreams),
		)
	}
}
