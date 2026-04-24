package errors

import (
	"errors"
	"testing"
)

// TestAsOrWrap tests the AsOrWrap function
func TestAsOrWrap_XError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantXErr  bool
		wantCode  Code
	}{
		{
			name:     "wrap_regular_error",
			err:      errors.New("test error"),
			wantXErr: true,
			wantCode: CodeInternal,
		},
		{
			name:     "unwrap_xerror",
			err:      New(CodeCfgInvalid, "invalid config", nil),
			wantXErr: true,
			wantCode: CodeCfgInvalid,
		},
		{
			name:     "wrap_nil_error_message",
			err:      errors.New(""),
			wantXErr: true,
			wantCode: CodeInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AsOrWrap(tt.err)

			if (result != nil) != tt.wantXErr {
				t.Errorf("AsOrWrap: got nil=%v, want XError", result == nil)
			}

			if result != nil && result.Code != tt.wantCode {
				t.Errorf("AsOrWrap: code = %q, want %q", result.Code, tt.wantCode)
			}

			// If original was XError with specific code, should preserve it
			if xe, ok := As(tt.err); ok {
				if result.Code != xe.Code {
					t.Errorf("AsOrWrap: should preserve original XError code")
				}
			}
		})
	}
}
