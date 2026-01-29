package db

import "testing"

func TestRegistry(t *testing.T) {
	type d struct{}
	name := "test_driver"
	Register(name, d{})
	if _, ok := Get(name); !ok {
		t.Fatalf("expected driver")
	}
}
