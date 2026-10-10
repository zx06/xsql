package mcp

import "testing"

func TestWriteAllowed(t *testing.T) {
	tests := []struct {
		name    string
		server  bool
		profile bool
		want    bool
	}{
		{name: "neither", server: false, profile: false, want: false},
		{name: "server only", server: true, profile: false, want: false},
		{name: "profile only", server: false, profile: true, want: false},
		{name: "both", server: true, profile: true, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := WriteAllowed(tt.server, tt.profile); got != tt.want {
				t.Fatalf("WriteAllowed(%v, %v) = %v, want %v", tt.server, tt.profile, got, tt.want)
			}
		})
	}
}
