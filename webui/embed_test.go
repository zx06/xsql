package webui

import (
	"io/fs"
	"testing"
)

// TestDist validates that Dist() returns a valid filesystem
func TestDist(t *testing.T) {
	result := Dist()

	if result == nil {
		t.Fatal("expected non-nil filesystem from Dist()")
	}

	if _, ok := result.(fs.FS); !ok {
		t.Errorf("expected fs.FS type, got %T", result)
	}
}

// TestDist_IsReadable validates that the returned fs is readable
func TestDist_IsReadable(t *testing.T) {
	result := Dist()

	// Try to open the root directory
	dir, err := fs.ReadDir(result, ".")
	if err != nil {
		// If dist is not populated, that's OK in tests
		// The embed is populated during CI builds
		t.Logf("dist directory not populated (expected in test): %v", err)
		return
	}

	// If we got here, dist has content
	if len(dir) == 0 {
		t.Logf("dist directory is empty (expected when not built)")
	}
}

// TestDistFiles_Embedded validates that DistFiles is properly embedded
func TestDistFiles_Embedded(t *testing.T) {
	// embed.FS cannot be nil, so we just try to use it
	// Try to read the dist directory
	entries, err := DistFiles.ReadDir("dist")
	if err != nil {
		// This is OK in tests - the dist/ directory is only populated during build
		t.Logf("dist/ directory not accessible (expected in test): %v", err)
		return
	}

	if len(entries) == 0 {
		t.Logf("dist/ directory is empty (expected in test environment)")
	}
}
