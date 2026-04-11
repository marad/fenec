package lua

import (
	"github.com/ollama/ollama/api"
	glua "github.com/yuin/gopher-lua"
)

// ArgsToLuaTable converts Ollama tool call arguments to a Lua table.
func ArgsToLuaTable(L *glua.LState, args api.ToolCallFunctionArguments) *glua.LTable {
	// TODO: implement
	_ = args
	return L.NewTable()
}
