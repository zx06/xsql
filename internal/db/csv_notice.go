package db

import "fmt"

// CSVTruncationNotice returns a stderr line when CSV output was truncated.
func (r *QueryResult) CSVTruncationNotice() (string, bool) {
	if r == nil || !r.Truncated {
		return "", false
	}
	reason := r.TruncatedReason
	if reason == "" {
		reason = TruncatedReasonMaxRows
	}
	msg := fmt.Sprintf("xsql: query result truncated (%s)", reason)
	if r.Hint != "" {
		msg += "; " + r.Hint
	}
	return msg, true
}
