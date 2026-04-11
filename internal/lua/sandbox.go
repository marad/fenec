package lua

import (
	"context"

	glua "github.com/yuin/gopher-lua"
)

// NewSandboxedState creates a sandboxed Lua VM with only safe libraries.
// Safe: base, table, string, math, package (for json require).
// Blocked: os, io, debug. Unsafe base functions dofile/loadfile are nil'd.
func NewSandboxedState(ctx context.Context) *glua.LState {
	// TODO: implement
	_ = ctx
	return glua.NewState()
}
