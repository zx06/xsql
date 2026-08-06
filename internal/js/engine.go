package js

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dop251/goja"

	"github.com/zx06/xsql/internal/errors"
	"github.com/zx06/xsql/internal/session"
)

type ExecutionResult struct {
	Value       any      `json:"value"`
	JSONString  string   `json:"json_string"`
	SummaryText string   `json:"summary_text"`
	Logs        []string `json:"logs"`
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

	// Inject console.log / console.error capture
	var logs []string
	console := vm.NewObject()
	_ = console.Set("log", func(call goja.FunctionCall) goja.Value {
		var args []string
		for _, arg := range call.Arguments {
			args = append(args, fmt.Sprintf("%v", arg.Export()))
		}
		logs = append(logs, strings.Join(args, " "))
		return goja.Undefined()
	})
	_ = console.Set("error", func(call goja.FunctionCall) goja.Value {
		var args []string
		for _, arg := range call.Arguments {
			args = append(args, fmt.Sprintf("%v", arg.Export()))
		}
		logs = append(logs, "[ERROR] "+strings.Join(args, " "))
		return goja.Undefined()
	})
	_ = vm.Set("console", console)

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
		return nil, errors.New(errors.CodeDBExecFailed, fmt.Sprintf("JavaScript execution error: %v", err), map[string]any{
			"err": err.Error(),
		})
	}

	if val == nil || goja.IsUndefined(val) || goja.IsNull(val) {
		summary := "(null)"
		if len(logs) > 0 {
			summary = strings.Join(logs, "\n")
		}
		return &ExecutionResult{
			Value:       nil,
			JSONString:  "null",
			SummaryText: summary,
			Logs:        logs,
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

	summaryText := jsonStr
	if len(logs) > 0 {
		summaryText = strings.Join(logs, "\n") + "\n\nReturn Value:\n" + jsonStr
	}

	return &ExecutionResult{
		Value:       exported,
		JSONString:  jsonStr,
		SummaryText: summaryText,
		Logs:        logs,
	}, nil
}
