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
				AppEnv:   "development",
				HTTPAddr: ":8080",
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
