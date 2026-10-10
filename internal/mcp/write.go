package mcp

// WriteAllowed reports whether MCP query tool may bypass read-only protections.
// Dual authorization matches CLI/TUI: both the MCP server entrypoint and the
// profile must explicitly opt in.
func WriteAllowed(serverAllowsWrite, profileAllowsWrite bool) bool {
	return serverAllowsWrite && profileAllowsWrite
}
