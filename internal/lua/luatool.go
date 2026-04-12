package lua

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"github.com/marad/fenec/internal/model"
	glua "github.com/yuin/gopher-lua"
	"github.com/yuin/gopher-lua/parse"
)

// LuaParam describes a single parameter for a Lua tool.
type LuaParam struct {
	Name        string
	Type        string // "string", "number", "boolean"
	Description string
	Required    bool
}

// LuaTool wraps a Lua script as a tool.Tool implementation.
// It stores pre-compiled bytecode so a fresh sandboxed LState can be created
// per execution without re-parsing the source.
type LuaTool struct {
	name        string
	description string
	params      []LuaParam
	scriptPath  string
	proto       *glua.FunctionProto
}

// CompileFile compiles a Lua source file to bytecode for reuse across LStates.
func CompileFile(path string) (*glua.FunctionProto, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	chunk, err := parse.Parse(reader, path)
	if err != nil {
		return nil, fmt.Errorf("%s: syntax error: %w", path, err)
	}

	proto, err := glua.Compile(chunk, path)
	if err != nil {
		return nil, fmt.Errorf("%s: compile error: %w", path, err)
	}
	return proto, nil
}

// NewLuaToolFromProto creates a LuaTool by executing the compiled bytecode
// in a temporary sandboxed LState to extract metadata (name, description,
// parameters, execute function). It validates that required fields are present.
func NewLuaToolFromProto(proto *glua.FunctionProto, scriptPath string) (*LuaTool, error) {
	// Create a temporary sandboxed LState to extract metadata.
	L := NewSandboxedState(context.Background())
	defer L.Close()

	// Execute the compiled script to get the returned table.
	fn := L.NewFunctionFromProto(proto)
	L.Push(fn)
	if err := L.PCall(0, 1, nil); err != nil {
		return nil, fmt.Errorf("%s: script error: %w", scriptPath, err)
	}

	// The script must return a table.
	ret := L.Get(-1)
	L.Pop(1)
	tbl, ok := ret.(*glua.LTable)
	if !ok {
		return nil, fmt.Errorf("%s: script must return a table, got %s", scriptPath, ret.Type())
	}

	// Validate required fields.
	nameVal := L.GetField(tbl, "name")
	if nameVal.Type() != glua.LTString || nameVal.String() == "" {
		return nil, fmt.Errorf("%s: missing or non-string 'name' field", scriptPath)
	}

	descVal := L.GetField(tbl, "description")
	if descVal.Type() != glua.LTString || descVal.String() == "" {
		return nil, fmt.Errorf("%s: missing or non-string 'description' field", scriptPath)
	}

	executeFn := L.GetField(tbl, "execute")
	if executeFn.Type() != glua.LTFunction {
		return nil, fmt.Errorf("%s: missing or non-function 'execute' field", scriptPath)
	}

	// Extract parameters (optional).
	var params []LuaParam
	paramsVal := L.GetField(tbl, "parameters")
	if paramsVal.Type() == glua.LTTable {
		paramsTbl := paramsVal.(*glua.LTable)
		paramsTbl.ForEach(func(_, value glua.LValue) {
			paramTbl, ok := value.(*glua.LTable)
			if !ok {
				return
			}
			p := LuaParam{
				Name:        fieldString(L, paramTbl, "name", ""),
				Type:        fieldString(L, paramTbl, "type", "string"),
				Description: fieldString(L, paramTbl, "description", ""),
				Required:    fieldBool(L, paramTbl, "required", false),
			}
			if p.Name != "" {
				params = append(params, p)
			}
		})
	}

	return &LuaTool{
		name:        nameVal.String(),
		description: descVal.String(),
		params:      params,
		scriptPath:  scriptPath,
		proto:       proto,
	}, nil
}

// Name returns the tool's unique identifier.
func (lt *LuaTool) Name() string {
	return lt.name
}

// Definition returns the tool definition for ChatRequest.Tools.
func (lt *LuaTool) Definition() model.ToolDefinition {
	props := make(map[string]model.ToolProperty)
	var required []string
	for _, p := range lt.params {
		props[p.Name] = model.ToolProperty{
			Type:        model.PropertyType{p.Type},
			Description: p.Description,
		}
		if p.Required {
			required = append(required, p.Name)
		}
	}
	return model.ToolDefinition{
		Type: "function",
		Function: model.ToolFunction{
			Name:        lt.name,
			Description: lt.description,
			Parameters: model.ToolFunctionParameters{
				Type:       "object",
				Required:   required,
				Properties: props,
			},
		},
	}
}

// Execute runs the Lua tool's execute function with the given arguments.
// A fresh sandboxed LState is created per invocation to prevent cross-call state pollution.
func (lt *LuaTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	L := NewSandboxedState(ctx)
	defer L.Close()

	// Load the pre-compiled script.
	fn := L.NewFunctionFromProto(lt.proto)
	L.Push(fn)
	if err := L.PCall(0, 1, nil); err != nil {
		return "", fmt.Errorf("lua tool %s: script error: %w", lt.name, err)
	}

	// Get the returned table and its execute function.
	tbl := L.CheckTable(-1)
	executeFn := L.GetField(tbl, "execute")
	if executeFn.Type() != glua.LTFunction {
		return "", fmt.Errorf("lua tool %s: execute is not a function", lt.name)
	}

	// Convert Go args to Lua table and call execute.
	argsTable := ArgsToLuaTable(L, args)
	if err := L.CallByParam(glua.P{
		Fn:      executeFn,
		NRet:    1,
		Protect: true,
	}, argsTable); err != nil {
		return "", fmt.Errorf("lua tool %s: execution error: %w", lt.name, err)
	}

	result := L.Get(-1)
	L.Pop(1)
	if result == glua.LNil {
		return "", nil
	}
	return glua.LVAsString(result), nil
}

// fieldString extracts a string field from a Lua table, returning def if missing or wrong type.
func fieldString(L *glua.LState, tbl *glua.LTable, key, def string) string {
	val := L.GetField(tbl, key)
	if val.Type() == glua.LTString {
		return val.String()
	}
	return def
}

// fieldBool extracts a boolean field from a Lua table, returning def if missing or wrong type.
func fieldBool(L *glua.LState, tbl *glua.LTable, key string, def bool) bool {
	val := L.GetField(tbl, key)
	if val.Type() == glua.LTBool {
		return bool(val.(glua.LBool))
	}
	return def
}
