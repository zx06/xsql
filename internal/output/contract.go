package output

import "github.com/zx06/xsql/internal/errors"

const SchemaVersion = 1

type ErrorObject struct {
	Code    errors.Code    `json:"code" yaml:"code"`
	Message string         `json:"message" yaml:"message"`
	Details map[string]any `json:"details,omitempty" yaml:"details,omitempty"`
}

type Envelope struct {
	OK            bool         `json:"ok" yaml:"ok"`
	SchemaVersion int          `json:"schema_version" yaml:"schema_version"`
	Error         *ErrorObject `json:"error,omitempty" yaml:"error,omitempty"`
	Data          any          `json:"data,omitempty" yaml:"data,omitempty"`
}
