package spec

import "testing"

func TestSpecCompiles(t *testing.T) {
	_ = Spec{SchemaVersion: 1}
}
