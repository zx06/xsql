package spec

import "github.com/zx06/xsql/internal/errors"

type FlagSpec struct {
	Name        string `json:"name" yaml:"name"`
	Shorthand   string `json:"shorthand,omitempty" yaml:"shorthand,omitempty"`
	Env         string `json:"env,omitempty" yaml:"env,omitempty"`
	Default     string `json:"default,omitempty" yaml:"default,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

type CommandSpec struct {
	Name        string     `json:"name" yaml:"name"`
	Description string     `json:"description,omitempty" yaml:"description,omitempty"`
	Flags       []FlagSpec `json:"flags,omitempty" yaml:"flags,omitempty"`
}

type Spec struct {
	SchemaVersion int           `json:"schema_version" yaml:"schema_version"`
	Commands      []CommandSpec `json:"commands" yaml:"commands"`
	ErrorCodes    []errors.Code `json:"error_codes" yaml:"error_codes"`
}
