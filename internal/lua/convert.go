package lua

import (
	"encoding/json"

	"github.com/ollama/ollama/api"
	glua "github.com/yuin/gopher-lua"
)

// ArgsToLuaTable converts Ollama tool call arguments to a Lua table.
// Supported Go types: string, float64, bool, nil. Complex types are JSON-encoded as strings.
func ArgsToLuaTable(L *glua.LState, args api.ToolCallFunctionArguments) *glua.LTable {
	tbl := L.NewTable()
	for key, val := range args.All() {
		switch v := val.(type) {
		case string:
			L.SetField(tbl, key, glua.LString(v))
		case float64:
			L.SetField(tbl, key, glua.LNumber(v))
		case bool:
			L.SetField(tbl, key, glua.LBool(v))
		case nil:
			L.SetField(tbl, key, glua.LNil)
		default:
			// For complex types (maps, slices), JSON-encode then set as string.
			b, _ := json.Marshal(v)
			L.SetField(tbl, key, glua.LString(string(b)))
		}
	}
	return tbl
}
