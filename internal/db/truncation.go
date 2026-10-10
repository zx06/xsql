package db

import (
	"strings"

	"github.com/zx06/xsql/internal/errors"
)

const (
	TruncatedReasonMaxRows          = "max_rows"
	TruncatedReasonMaxResponseBytes = "max_response_bytes"
)

// AppendTruncatedReason merges a truncation reason token into any existing value.
func AppendTruncatedReason(existing, reason string) string {
	if reason == "" {
		return existing
	}
	if existing == "" {
		return reason
	}
	if strings.Contains(existing, reason) {
		return existing
	}
	return existing + "," + reason
}

// RefreshTruncationHint sets Hint from TruncatedReason when truncated.
func RefreshTruncationHint(r *QueryResult) {
	if r == nil || !r.Truncated {
		return
	}
	r.Hint = errors.BuildTruncationHint(r.TruncatedReason)
}
