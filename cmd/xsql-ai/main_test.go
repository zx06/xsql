package main

import (
	"testing"
)

func TestAIFlags_Parsing(t *testing.T) {
	flags := &AIFlags{
		Profile:          "dev",
		Model:            "gpt-4o",
		BaseURL:          "https://api.openai.com/v1",
		APIKey:           "sk-test",
		UnsafeAllowWrite: true,
		Prompt:           "Count users",
	}

	if flags.Profile != "dev" {
		t.Errorf("expected profile=dev, got %s", flags.Profile)
	}
	if !flags.UnsafeAllowWrite {
		t.Error("expected UnsafeAllowWrite to be true")
	}
	if flags.Prompt != "Count users" {
		t.Errorf("expected prompt='Count users', got %s", flags.Prompt)
	}
}
