package config

import (
	"testing"
)

// TestProfileToInfo tests ProfileToInfo conversion function
func TestProfileToInfo(t *testing.T) {
	tests := []struct {
		name string
		prof Profile
		wantMode string
	}{
		{
			name: "read_only_default",
			prof: Profile{
				DB:          "mysql",
				Description: "Test profile",
			},
			wantMode: "read-only",
		},
		{
			name: "read_write_enabled",
			prof: Profile{
				DB:                "mysql",
				Description:       "Test profile",
				UnsafeAllowWrite:  true,
			},
			wantMode: "read-write",
		},
		{
			name: "read_write_false",
			prof: Profile{
				DB:                "postgresql",
				Description:       "PG profile",
				UnsafeAllowWrite:  false,
			},
			wantMode: "read-only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := ProfileToInfo("test_profile", tt.prof)

			if info.Name != "test_profile" {
				t.Errorf("Name: got %q, want 'test_profile'", info.Name)
			}
			if info.DB != tt.prof.DB {
				t.Errorf("DB: got %q, want %q", info.DB, tt.prof.DB)
			}
			if info.Description != tt.prof.Description {
				t.Errorf("Description: got %q, want %q", info.Description, tt.prof.Description)
			}
			if info.Mode != tt.wantMode {
				t.Errorf("Mode: got %q, want %q", info.Mode, tt.wantMode)
			}
		})
	}
}
