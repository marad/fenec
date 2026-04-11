package lua

import (
	"context"

	ljson "github.com/vadv/gopher-lua-libs/json"
	glua "github.com/yuin/gopher-lua"
)

// NewSandboxedState creates a sandboxed Lua VM with only safe libraries.
// Safe: base, table, string, math, package (for json require).
// Blocked: os, io, debug. Unsafe base functions dofile/loadfile are nil'd.
// The context is set on the LState to enforce execution timeouts.
func NewSandboxedState(ctx context.Context) *glua.LState {
	L := glua.NewState(glua.Options{SkipOpenLibs: true})

	// Open only safe libraries. Package (LoadLib) is needed for require().
	for _, pair := range []struct {
		name   string
		opener glua.LGFunction
	}{
		{glua.LoadLibName, glua.OpenPackage},
		{glua.BaseLibName, glua.OpenBase},
		{glua.TabLibName, glua.OpenTable},
		{glua.StringLibName, glua.OpenString},
		{glua.MathLibName, glua.OpenMath},
	} {
		if err := L.CallByParam(glua.P{
			Fn:      L.NewFunction(pair.opener),
			NRet:    0,
			Protect: true,
		}, glua.LString(pair.name)); err != nil {
			panic("failed to open safe Lua library " + pair.name + ": " + err.Error())
		}
	}

	// Remove unsafe base functions that OpenBase exposes.
	L.SetGlobal("dofile", glua.LNil)
	L.SetGlobal("loadfile", glua.LNil)

	// Set context for timeout enforcement via gopher-lua's instruction check.
	L.SetContext(ctx)

	// Preload JSON module so Lua scripts can require("json").
	ljson.Preload(L)

	return L
}
