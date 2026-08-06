package js

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dop251/goja"

	"github.com/zx06/xsql/internal/errors"
	"github.com/zx06/xsql/internal/session"
)

type ExecutionResult struct {
	Value       any    `json:"value"`
	JSONString  string `json:"json_string"`
	SummaryText string `json:"summary_text"`
}

type JSEngine struct {
	DefaultTimeout time.Duration
}

func NewJSEngine(timeout time.Duration) *JSEngine {
	if timeout <= 0 {
		timeout = 1 * time.Minute
	}
	return &JSEngine{
		DefaultTimeout: timeout,
	}
}

func (e *JSEngine) Execute(ctx context.Context, jsCode string, store *session.SessionDataStore) (*ExecutionResult, *errors.XError) {
	if ctx == nil {
		ctx = context.Background()
	}

	execCtx, cancel := context.WithTimeout(ctx, e.DefaultTimeout)
	defer cancel()

	vm := goja.New()
	vm.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))

	// Inject all active datasets from store
	if store != nil {
		allData := store.GetAll()
		for id, queryRes := range allData {
			if queryRes != nil {
				_ = vm.Set(id, queryRes.Rows)
			}
		}
		// Inject latest query result as `rows` and `columns`
		if latest, ok := store.Latest(); ok && latest != nil {
			_ = vm.Set("rows", latest.Rows)
			_ = vm.Set("columns", latest.Columns)
		}
	}

	// Timeout interrupt setup
	doneChan := make(chan struct{})
	defer close(doneChan)

	go func() {
		select {
		case <-execCtx.Done():
			if execCtx.Err() == context.DeadlineExceeded {
				vm.Interrupt("execution timeout (limit reached)")
			}
		case <-doneChan:
		}
	}()

	// Execute JS script
	val, err := vm.RunString(jsCode)
	if err != nil {
		return nil, errors.New(errors.CodeDBExecFailed, "JavaScript execution error", map[string]any{
			"err": err.Error(),
		})
	}

	if val == nil || goja.IsUndefined(val) || goja.IsNull(val) {
		return &ExecutionResult{
			Value:       nil,
			JSONString:  "null",
			SummaryText: "(null)",
		}, nil
	}

	exported := val.Export()
	jsonBytes, jsonErr := json.MarshalIndent(exported, "", "  ")
	jsonStr := ""
	if jsonErr == nil {
		jsonStr = string(jsonBytes)
	} else {
		jsonStr = fmt.Sprintf("%v", exported)
	}

	return &ExecutionResult{
		Value:       exported,
		JSONString:  jsonStr,
		SummaryText: jsonStr,
	}, nil
}
