package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/marad/fenec/internal/model"
)

// EditResult holds the structured output of a file edit operation.
type EditResult struct {
	Status       string `json:"status"`
	LinesChanged int    `json:"lines_changed"`
	Context      string `json:"context"`
}

// EditFileTool performs search-and-replace on files.
// Edits inside CWD need no approval; edits outside CWD are gated by ApproverFunc.
// Edits to denied paths are rejected before any approval check.
type EditFileTool struct {
	approver ApproverFunc
}

// NewEditFileTool creates an EditFileTool with the given approver.
// If approver is nil, out-of-CWD edits are denied.
func NewEditFileTool(approver ApproverFunc) *EditFileTool {
	return &EditFileTool{approver: approver}
}

// Name returns the tool identifier used for dispatch.
func (e *EditFileTool) Name() string {
	return "edit_file"
}

// Definition returns the tool definition for ChatRequest.Tools.
func (e *EditFileTool) Definition() model.ToolDefinition {
	return model.ToolDefinition{
		Type: "function",
		Function: model.ToolFunction{
			Name:        "edit_file",
			Description: "Edit a file by replacing the first occurrence of old_text with new_text. Returns the number of lines changed and surrounding context.",
			Parameters: model.ToolFunctionParameters{
				Type:     "object",
				Required: []string{"path", "old_text", "new_text"},
				Properties: map[string]model.ToolProperty{
					"path": {
						Type:        model.PropertyType{"string"},
						Description: "Path to the file to edit. File must exist.",
					},
					"old_text": {
						Type:        model.PropertyType{"string"},
						Description: "Exact text to find in the file. First occurrence will be replaced.",
					},
					"new_text": {
						Type:        model.PropertyType{"string"},
						Description: "Text to replace old_text with.",
					},
				},
			},
		},
	}
}

// Execute performs the search-and-replace on the file specified by the path argument.
func (e *EditFileTool) Execute(_ context.Context, args map[string]any) (string, error) {
	pathVal, ok := args["path"]
	if !ok {
		return "", fmt.Errorf("missing required argument: path")
	}
	path, ok := pathVal.(string)
	if !ok {
		return "", fmt.Errorf("missing required argument: path")
	}

	oldTextVal, ok := args["old_text"]
	if !ok {
		return "", fmt.Errorf("missing required argument: old_text")
	}
	oldText, ok := oldTextVal.(string)
	if !ok {
		return "", fmt.Errorf("missing required argument: old_text")
	}

	newTextVal, ok := args["new_text"]
	if !ok {
		return "", fmt.Errorf("missing required argument: new_text")
	}
	newText, ok := newTextVal.(string)
	if !ok {
		return "", fmt.Errorf("missing required argument: new_text")
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
		if e.approver == nil {
			return "", fmt.Errorf("edit denied: no approver configured")
		}
		if !e.approver(fmt.Sprintf("edit file outside working directory: %s", absPath)) {
			return "", fmt.Errorf("edit denied by user: %s", absPath)
		}
	}

	// Resolve path for file operations.
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	// Check file exists (D-20: tool result error, not Go error).
	info, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		return errorJSON("file does not exist: %s", path), nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to stat file: %w", err)
	}
	originalMode := info.Mode()

	// Read file as raw bytes to preserve line endings (CRLF, etc.).
	raw, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	content := string(raw)

	// Check if old_text exists in the file (D-19).
	if !strings.Contains(content, oldText) {
		return errorJSON("text not found in file"), nil
	}

	// Find the byte offset of the match (for context extraction).
	matchOffset := strings.Index(content, oldText)

	// Replace first occurrence only (D-18).
	updated := strings.Replace(content, oldText, newText, 1)

	// Write updated content preserving original file permissions.
	if err := os.WriteFile(absPath, []byte(updated), originalMode.Perm()); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	// Calculate lines changed.
	oldLines := strings.Count(oldText, "\n")
	newLines := strings.Count(newText, "\n")
	linesChanged := absInt(newLines-oldLines) + 1

	// Extract context around the edit point.
	contextStr := extractContext(updated, matchOffset, 3)

	result := EditResult{
		Status:       "ok",
		LinesChanged: linesChanged,
		Context:      contextStr,
	}
	b, _ := json.Marshal(result)
	return string(b), nil
}

// absInt returns the absolute value of an integer.
func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// extractContext extracts surrounding lines around a byte offset in content.
// Returns surroundLines before and after the edit point, joined with newlines.
func extractContext(content string, byteOffset int, surroundLines int) string {
	lines := strings.Split(content, "\n")

	// Find which line contains the byte offset.
	pos := 0
	editLine := 0
	for i, line := range lines {
		lineEnd := pos + len(line)
		if i < len(lines)-1 {
			lineEnd++ // account for the \n separator
		}
		if byteOffset < lineEnd {
			editLine = i
			break
		}
		pos = lineEnd
	}

	// Calculate range.
	start := editLine - surroundLines
	if start < 0 {
		start = 0
	}
	end := editLine + surroundLines + 1
	if end > len(lines) {
		end = len(lines)
	}

	return strings.Join(lines[start:end], "\n")
}
