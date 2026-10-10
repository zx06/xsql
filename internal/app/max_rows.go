package app

import (
	"strconv"
	"strings"

	"github.com/zx06/xsql/internal/config"
	"github.com/zx06/xsql/internal/errors"
)

const (
	// DefaultQueryMaxRows is the default row cap for CLI, Web, and TUI queries.
	DefaultQueryMaxRows = 1000
	// DefaultMCPQueryMaxRows is a lower default for MCP tool responses (smaller agent payloads).
	DefaultMCPQueryMaxRows = 500
	// DefaultMCPMaxRowsLimit is the hard ceiling for MCP query max_rows (requests above are clamped).
	DefaultMCPMaxRowsLimit = 10000
	// DefaultMCPMaxResponseBytes is the default serialized JSON size cap for MCP query results.
	DefaultMCPMaxResponseBytes = 512 * 1024
)

// QueryTruncationHint is returned when results are truncated for agents/humans.
const QueryTruncationHint = errors.HintQueryResultTruncated

// MCPResponseTruncationHint is appended when MCP byte guard trims rows.
const MCPResponseTruncationHint = errors.HintMCPResponseTruncated

// MCPMaxRowsMeta describes MCP max_rows resolution/clamping for response metadata.
type MCPMaxRowsMeta struct {
	Requested int
	Clamped   bool
	Limit     int
}

// ResolveQueryMaxRows returns the effective row limit for CLI/Web/TUI.
// limit 0 means unlimited. Negative values are invalid.
func ResolveQueryMaxRows(profile config.Profile, envMaxRows string, cliValue int, cliSet bool) (limit int, xe *errors.XError) {
	if cliSet {
		if cliValue < 0 {
			return 0, errors.New(errors.CodeCfgInvalid, "max_rows must be >= 0", map[string]any{"max_rows": cliValue})
		}
		return cliValue, nil
	}
	if strings.TrimSpace(envMaxRows) != "" {
		v, err := strconv.Atoi(strings.TrimSpace(envMaxRows))
		if err != nil || v < 0 {
			return 0, errors.New(errors.CodeCfgInvalid, "XSQL_MAX_ROWS must be a non-negative integer", map[string]any{"value": envMaxRows})
		}
		return v, nil
	}
	if profile.MaxRows > 0 {
		return profile.MaxRows, nil
	}
	return DefaultQueryMaxRows, nil
}

// ResolveMCPMaxRowsLimit returns the configured MCP max_rows ceiling.
func ResolveMCPMaxRowsLimit(mcpCfg config.MCPConfig) int {
	if mcpCfg.MaxRowsLimit > 0 {
		return mcpCfg.MaxRowsLimit
	}
	return DefaultMCPMaxRowsLimit
}

// ResolveMCPQueryMaxRows returns the MCP row limit (never unlimited).
// MCP max_rows 0 means unset (use defaults). Negative values are rejected.
func ResolveMCPQueryMaxRows(profile config.Profile, mcpCfg config.MCPConfig, argValue int, argSet bool) (effective int, meta MCPMaxRowsMeta, xe *errors.XError) {
	if argSet {
		if argValue < 0 {
			return 0, meta, errors.New(errors.CodeCfgInvalid, "max_rows must be >= 0 for MCP query", map[string]any{"max_rows": argValue})
		}
		if argValue == 0 {
			argSet = false
		}
	}

	resolved := 0
	if argSet && argValue > 0 {
		resolved = argValue
		meta.Requested = argValue
	} else if profile.MaxRows > 0 {
		resolved = profile.MaxRows
	} else if mcpCfg.MaxRows > 0 {
		resolved = mcpCfg.MaxRows
	} else {
		resolved = DefaultMCPQueryMaxRows
	}

	ceiling := ResolveMCPMaxRowsLimit(mcpCfg)
	if resolved > ceiling {
		if meta.Requested == 0 {
			meta.Requested = resolved
		}
		meta.Clamped = true
		meta.Limit = ceiling
		resolved = ceiling
	}
	return resolved, meta, nil
}

// ResolveMCPMaxResponseBytes returns the MCP JSON response byte cap.
func ResolveMCPMaxResponseBytes(mcpCfg config.MCPConfig) int {
	if mcpCfg.MaxResponseBytes > 0 {
		return mcpCfg.MaxResponseBytes
	}
	return DefaultMCPMaxResponseBytes
}
