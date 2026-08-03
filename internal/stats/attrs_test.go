package stats

import (
	"testing"
)

func TestParseAttrs_Empty(t *testing.T) {
	result := ParseAttrs(nil, "")
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestParseAttrs_ENVOnly(t *testing.T) {
	result := ParseAttrs(nil, "env=prod,team=ai")
	if len(result) != 2 {
		t.Fatalf("expected 2 attrs, got %d", len(result))
	}
	if result["env"] != "prod" {
		t.Errorf("expected env=prod, got %v", result["env"])
	}
	if result["team"] != "ai" {
		t.Errorf("expected team=ai, got %v", result["team"])
	}
}

func TestParseAttrs_CLIOnly(t *testing.T) {
	result := ParseAttrs([]string{"env=dev", "agent=cursor"}, "")
	if len(result) != 2 {
		t.Fatalf("expected 2 attrs, got %d", len(result))
	}
	if result["env"] != "dev" {
		t.Errorf("expected env=dev, got %v", result["env"])
	}
	if result["agent"] != "cursor" {
		t.Errorf("expected agent=cursor, got %v", result["agent"])
	}
}

func TestParseAttrs_CLIOverridesENV(t *testing.T) {
	result := ParseAttrs([]string{"env=dev"}, "env=prod,team=ai")
	if len(result) != 2 {
		t.Fatalf("expected 2 attrs, got %d", len(result))
	}
	if result["env"] != "dev" {
		t.Errorf("expected env=dev (CLI override), got %v", result["env"])
	}
	if result["team"] != "ai" {
		t.Errorf("expected team=ai, got %v", result["team"])
	}
}

func TestParseAttrs_Spaces(t *testing.T) {
	result := ParseAttrs([]string{" env = dev "}, " team = ai ")
	if len(result) != 2 {
		t.Fatalf("expected 2 attrs, got %d", len(result))
	}
	if result["env"] != "dev" {
		t.Errorf("expected env=dev, got %v", result["env"])
	}
	if result["team"] != "ai" {
		t.Errorf("expected team=ai, got %v", result["team"])
	}
}

func TestParseAttrs_InvalidFormat(t *testing.T) {
	result := ParseAttrs([]string{"noequals"}, "")
	if result != nil {
		t.Errorf("expected nil for invalid format, got %v", result)
	}
}

func TestValidateAttr_Valid(t *testing.T) {
	if err := ValidateAttr("env", "prod"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := ValidateAttr("my_key-1", "value"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateAttr_EmptyKey(t *testing.T) {
	if err := ValidateAttr("", "prod"); err == nil {
		t.Error("expected error for empty key")
	}
}

func TestValidateAttr_EmptyValue(t *testing.T) {
	if err := ValidateAttr("env", ""); err == nil {
		t.Error("expected error for empty value")
	}
}

func TestValidateAttr_InvalidKey(t *testing.T) {
	if err := ValidateAttr("env key", "prod"); err == nil {
		t.Error("expected error for key with space")
	}
	if err := ValidateAttr("env.key", "prod"); err == nil {
		t.Error("expected error for key with dot")
	}
}

func TestValidateAttrs(t *testing.T) {
	if err := ValidateAttrs(map[string]string{"env": "prod", "team": "ai"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := ValidateAttrs(map[string]string{"": "prod"}); err == nil {
		t.Error("expected error for empty key")
	}
}
