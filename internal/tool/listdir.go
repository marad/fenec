package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/ollama/ollama/api"
)

// DirEntry describes a single entry in a directory listing.
type DirEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

// ListDirTool lists directory contents with sorted output.
type ListDirTool struct{}

// NewListDirTool creates a ListDirTool.
func NewListDirTool() *ListDirTool {
	return &ListDirTool{}
}

// Name returns the tool identifier used for dispatch.
func (l *ListDirTool) Name() string {
	return "list_directory"
}

// Definition returns the Ollama API tool definition for ChatRequest.Tools.
func (l *ListDirTool) Definition() api.Tool {
	props := api.NewToolPropertiesMap()
	props.Set("path", api.ToolProperty{
		Type:        api.PropertyType{"string"},
		Description: "Path to the directory to list",
	})

	return api.Tool{
		Type: "function",
		Function: api.ToolFunction{
			Name:        "list_directory",
			Description: "List the contents of a directory, returning entries sorted with directories first then files, each with name, type, and size.",
			Parameters: api.ToolFunctionParameters{
				Type:       "object",
				Required:   []string{"path"},
				Properties: props,
			},
		},
	}
}

// Execute lists the directory specified by the path argument.
func (l *ListDirTool) Execute(_ context.Context, args api.ToolCallFunctionArguments) (string, error) {
	pathVal, ok := args.Get("path")
	if !ok {
		return "", fmt.Errorf("missing required argument: path")
	}
	path, ok := pathVal.(string)
	if !ok {
		return "", fmt.Errorf("missing required argument: path")
	}

	// Check deny list before any directory I/O.
	denied, err := IsDeniedPath(path)
	if err != nil {
		return errorJSON("access denied: path is in restricted area"), nil
	}
	if denied {
		return errorJSON("access denied: path is in restricted area"), nil
	}

	dirEntries, err := os.ReadDir(path)
	if err != nil {
		return "", fmt.Errorf("failed to read directory: %w", err)
	}

	entries := make([]DirEntry, 0, len(dirEntries))
	for _, de := range dirEntries {
		entry := DirEntry{
			Name:  de.Name(),
			IsDir: de.IsDir(),
		}
		if !de.IsDir() {
			info, infoErr := de.Info()
			if infoErr != nil {
				// Skip entries we can't stat (permission errors, etc.)
				continue
			}
			entry.Size = info.Size()
		}
		entries = append(entries, entry)
	}

	// Sort: directories first (alphabetically), then files (alphabetically).
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir // dirs before files
		}
		return entries[i].Name < entries[j].Name
	})

	b, err := json.Marshal(entries)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}
	return string(b), nil
}
