package errors

import (
	stderrors "errors"
	"fmt"
)

// XError 是结构化错误，满足 docs/error-contract.md。
type XError struct {
	Code    Code           `json:"code" yaml:"code"`
	Message string         `json:"message" yaml:"message"`
	Details map[string]any `json:"details,omitempty" yaml:"details,omitempty"`
	cause   error
}

func (e *XError) Error() string {
	if e == nil {
		return ""
	}
	if e.cause == nil {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
}

func (e *XError) Unwrap() error { return e.cause }

func New(code Code, message string, details map[string]any) *XError {
	return &XError{Code: code, Message: message, Details: details}
}

func Wrap(code Code, message string, details map[string]any, cause error) *XError {
	return &XError{Code: code, Message: message, Details: details, cause: cause}
}

func As(err error) (*XError, bool) {
	var xe *XError
	if stderrors.As(err, &xe) {
		return xe, true
	}
	return nil, false
}

func AsOrWrap(err error) *XError {
	if xe, ok := As(err); ok {
		return xe
	}
	return Wrap(CodeInternal, err.Error(), nil, err)
}
