package output

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/zx06/xsql/internal/errors"
)

func TestWriteOK_JSONEnvelope(t *testing.T) {
	var out bytes.Buffer
	w := New(&out, &bytes.Buffer{})
	if err := w.WriteOK(FormatJSON, map[string]any{"k": "v"}); err != nil {
		t.Fatal(err)
	}
	var env Envelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if !env.OK || env.SchemaVersion != SchemaVersion {
		t.Fatalf("unexpected envelope: %+v", env)
	}
}

func TestWriteError_JSONEnvelope(t *testing.T) {
	var out bytes.Buffer
	w := New(&out, &bytes.Buffer{})
	xe := errors.New(errors.CodeCfgInvalid, "bad", map[string]any{"x": 1})
	if err := w.WriteError(FormatJSON, xe); err != nil {
		t.Fatal(err)
	}
	var env Envelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.OK || env.Error == nil || env.Error.Code != errors.CodeCfgInvalid {
		t.Fatalf("unexpected envelope: %+v", env)
	}
}
