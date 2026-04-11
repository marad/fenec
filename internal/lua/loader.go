package lua

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadError records a tool that failed to load.
type LoadError struct {
	Path   string // Absolute path to the .lua file
	Reason string // Human-readable error description
}

// Error implements the error interface for LoadError.
func (e *LoadError) Error() string {
	return fmt.Sprintf("%s: %s", e.Path, e.Reason)
}

// LoadResult holds the outcome of scanning a tools directory.
type LoadResult struct {
	Tools  []*LuaTool  // Successfully loaded tools
	Errors []LoadError // Tools that failed to load
}

// LoadTools scans toolsDir for .lua files, compiles and validates each,
// and returns successfully loaded tools alongside errors for broken ones.
// A missing directory is treated as zero tools (no error).
func LoadTools(toolsDir string) (*LoadResult, error) {
	result := &LoadResult{}

	entries, err := os.ReadDir(toolsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".lua") {
			continue
		}

		path := filepath.Join(toolsDir, entry.Name())

		proto, err := CompileFile(path)
		if err != nil {
			result.Errors = append(result.Errors, LoadError{
				Path:   path,
				Reason: err.Error(),
			})
			continue
		}

		lt, err := NewLuaToolFromProto(proto, path)
		if err != nil {
			result.Errors = append(result.Errors, LoadError{
				Path:   path,
				Reason: err.Error(),
			})
			continue
		}

		result.Tools = append(result.Tools, lt)
	}

	return result, nil
}
