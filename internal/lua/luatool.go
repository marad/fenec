package lua

import (
	"context"
	"fmt"

	"github.com/ollama/ollama/api"
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
type LuaTool struct {
	name        string
	description string
	params      []LuaParam
	scriptPath  string
	proto       *glua.FunctionProto
}

// CompileFile compiles a Lua source file to bytecode for reuse.
func CompileFile(path string) (*glua.FunctionProto, error) {
	// TODO: implement
	_, _ = parse.Parse, fmt.Errorf
	return nil, fmt.Errorf("not implemented")
}

// NewLuaToolFromProto creates a LuaTool by executing the proto to extract metadata.
func NewLuaToolFromProto(proto *glua.FunctionProto, scriptPath string) (*LuaTool, error) {
	// TODO: implement
	return nil, fmt.Errorf("not implemented")
}

// Name returns the tool's unique identifier.
func (lt *LuaTool) Name() string {
	return lt.name
}

// Definition returns the Ollama API tool definition.
func (lt *LuaTool) Definition() api.Tool {
	// TODO: implement
	return api.Tool{}
}

// Execute runs the Lua tool's execute function with the given arguments.
func (lt *LuaTool) Execute(ctx context.Context, args api.ToolCallFunctionArguments) (string, error) {
	// TODO: implement
	return "", fmt.Errorf("not implemented")
}
