package errors

import (
	"strings"
	"testing"
)

func TestBuildTruncationHint(t *testing.T) {
	if got := BuildTruncationHint("max_rows"); !strings.Contains(got, HintQueryResultTruncated) {
		t.Fatalf("got %q", got)
	}
	if got := BuildTruncationHint("max_response_bytes"); !strings.Contains(got, HintMCPResponseTruncated) {
		t.Fatalf("got %q", got)
	}
	got := BuildTruncationHint("max_rows,max_response_bytes")
	if !strings.Contains(got, HintQueryResultTruncated) || !strings.Contains(got, HintMCPResponseTruncated) {
		t.Fatalf("got %q", got)
	}
}
