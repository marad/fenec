package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/marad/fenec/internal/model"
)

// WriteFileTool creates or overwrites files with content.
// Writes inside CWD need no approval; writes outside CWD are gated by ApproverFunc.
// Writes to denied paths are rejected before any approval check.
type WriteFileTool struct {
	approver ApproverFunc
}

// NewWriteFileTool creates a WriteFileTool with the given approver.
// If approver is nil, out-of-CWD writes are denied.
func NewWriteFileTool(approver ApproverFunc) *WriteFileTool {
	return &WriteFileTool{approver: approver}
}

// Name returns the tool identifier used for dispatch.
func (w *WriteFileTool) Name() string {
	return "write_file"
}

// Definition returns the tool definition for ChatRequest.Tools.
func (w *WriteFileTool) Definition() model.ToolDefinition {
	return model.ToolDefinition{
		Type: "function",
		Function: model.ToolFunction{
			Name:        "write_file",
			Description: "Write content to a file, creating it if it does not exist or overwriting if it does. Creates parent directories automatically.",
			Parameters: model.ToolFunctionParameters{
				Type:     "object",
				Required: []string{"path", "content"},
				Properties: map[string]model.ToolProperty{
					"path": {
						Type:        model.PropertyType{"string"},
						Description: "Path to the file to write. Creates parent directories if needed.",
					},
					"content": {
						Type:        model.PropertyType{"string"},
						Description: "The content to write to the file. Overwrites existing content.",
					},
				},
			},
		},
	}
}

// Execute writes content to the file specified by the path argument.
func (w *WriteFileTool) Execute(_ context.Context, args map[string]any) (string, error) {
	pathVal, ok := args["path"]
	if !ok {
		return "", fmt.Errorf("missing required argument: path")
	}
	path, ok := pathVal.(string)
	if !ok {
		return "", fmt.Errorf("missing required argument: path")
	}

	contentVal, ok := args["content"]
	if !ok {
		return "", fmt.Errorf("missing required argument: content")
	}
	content, ok := contentVal.(string)
	if !ok {
		return "", fmt.Errorf("missing required argument: content")
	}

	// Check deny list before any approval check (D-07).
	denied, err := IsDeniedPath(path)
	if err != nil {
		return errorJSON("access denied: path is in restricted area"), nil
	}
	if denied {
		return errorJSON("access denied: path is in restricted area"), nil
	}

	// Check whether path is outside CWD.
	outside, err := IsOutsideCWD(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}
	if outside {
		absPath, _ := filepath.Abs(path)
		if w.approver == nil {
			return "", fmt.Errorf("write denied: no approver configured")
		}
		if !w.approver(fmt.Sprintf("write file outside working directory: %s", absPath)) {
			return "", fmt.Errorf("write denied by user: %s", absPath)
		}
	}

	// Resolve absolute path for writing.
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	// Create parent directories (D-15: mkdir -p).
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create directories: %w", err)
	}

	// Write file content.
	if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	// Return success JSON.
	result := map[string]interface{}{
		"status":        "ok",
		"path":          absPath,
		"bytes_written": len(content),
	}
	b, _ := json.Marshal(result)
	return string(b), nil
}
