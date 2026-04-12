package tool

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/marad/fenec/internal/model"
)

const defaultReadLimit = 1000

// ReadResult holds the structured output of a file read operation.
type ReadResult struct {
	Content    string `json:"content"`
	TotalLines int    `json:"total_lines"`
	LinesShown int    `json:"lines_shown"`
	Truncated  bool   `json:"truncated"`
}

// ReadFileTool reads file contents with optional offset and limit.
type ReadFileTool struct{}

// NewReadFileTool creates a ReadFileTool.
func NewReadFileTool() *ReadFileTool {
	return &ReadFileTool{}
}

// Name returns the tool identifier used for dispatch.
func (r *ReadFileTool) Name() string {
	return "read_file"
}

// Definition returns the tool definition for ChatRequest.Tools.
func (r *ReadFileTool) Definition() model.ToolDefinition {
	return model.ToolDefinition{
		Type: "function",
		Function: model.ToolFunction{
			Name:        "read_file",
			Description: "Read the contents of a file and return it with line count metadata. Supports offset and limit for partial reads.",
			Parameters: model.ToolFunctionParameters{
				Type:     "object",
				Required: []string{"path"},
				Properties: map[string]model.ToolProperty{
					"path": {
						Type:        model.PropertyType{"string"},
						Description: "Absolute or relative path to the file to read",
					},
					"offset": {
						Type:        model.PropertyType{"integer"},
						Description: "Start reading from this line number (0-based). Optional.",
					},
					"limit": {
						Type:        model.PropertyType{"integer"},
						Description: "Maximum number of lines to read. Optional, defaults to 1000.",
					},
				},
			},
		},
	}
}

// Execute reads the file specified by the path argument.
func (r *ReadFileTool) Execute(_ context.Context, args map[string]any) (string, error) {
	pathVal, ok := args["path"]
	if !ok {
		return "", fmt.Errorf("missing required argument: path")
	}
	path, ok := pathVal.(string)
	if !ok {
		return "", fmt.Errorf("missing required argument: path")
	}

	// Check deny list before any file I/O.
	denied, err := IsDeniedPath(path)
	if err != nil {
		return errorJSON("access denied: path is in restricted area"), nil
	}
	if denied {
		return errorJSON("access denied: path is in restricted area"), nil
	}

	// Check for binary file.
	binary, err := isBinaryFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to check file: %w", err)
	}
	if binary {
		return errorJSON("binary file detected, use shell_exec for binary inspection"), nil
	}

	offset, err := getOptionalInt(args, "offset", 0)
	if err != nil {
		return "", fmt.Errorf("invalid offset: %w", err)
	}
	limit, err := getOptionalInt(args, "limit", defaultReadLimit)
	if err != nil {
		return "", fmt.Errorf("invalid limit: %w", err)
	}

	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var collected []string
	totalLines := 0
	for scanner.Scan() {
		if totalLines >= offset && len(collected) < limit {
			collected = append(collected, scanner.Text())
		}
		totalLines++
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading file: %w", err)
	}

	// Truncated is true when there are more lines available beyond what was returned.
	availableLines := totalLines - offset
	if availableLines < 0 {
		availableLines = 0
	}
	result := ReadResult{
		Content:    strings.Join(collected, "\n"),
		TotalLines: totalLines,
		LinesShown: len(collected),
		Truncated:  len(collected) < availableLines,
	}

	b, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}
	return string(b), nil
}

// isBinaryFile checks whether a file contains null bytes in its first 512 bytes.
func isBinaryFile(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return false, err
	}

	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			return true, nil
		}
	}
	return false, nil
}

// getOptionalInt extracts an optional integer argument from tool call args.
// JSON numbers arrive as float64 in Go; this handles the type assertion.
func getOptionalInt(args map[string]any, key string, defaultVal int) (int, error) {
	val, ok := args[key]
	if !ok {
		return defaultVal, nil
	}

	switch v := val.(type) {
	case float64:
		return int(v), nil
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return 0, fmt.Errorf("invalid integer for %s: %w", key, err)
		}
		return int(i), nil
	case int:
		return v, nil
	case int64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("unexpected type for %s: %T", key, val)
	}
}
