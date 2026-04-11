package tool

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// deniedPrefixes is the resolved list of path prefixes that are never accessible.
// Built in init() so os.UserHomeDir is called once at startup.
var deniedPrefixes []string

func init() {
	deniedPrefixes = []string{
		"/etc",
		"/usr",
		"/bin",
		"/sbin",
		"/boot",
	}

	home, err := os.UserHomeDir()
	if err == nil {
		deniedPrefixes = append(deniedPrefixes,
			filepath.Join(home, ".ssh"),
			filepath.Join(home, ".gnupg"),
		)
	}
}

// IsDeniedPath checks whether a path falls within any denied prefix.
// It resolves symlinks to prevent bypass. Returns (true, nil) if denied,
// (false, nil) if allowed. On resolution failure, returns (true, err) — fail closed.
func IsDeniedPath(path string) (bool, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return true, err
	}

	// Resolve symlinks. If the path doesn't exist yet (e.g. new file for write),
	// walk up the directory tree to find the first existing ancestor, resolve it,
	// then append the remaining unresolved components. This handles mkdir -p
	// scenarios where multiple parent directories don't exist yet.
	resolved, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		resolved, err = resolveWithAncestor(absPath)
		if err != nil {
			return true, err
		}
	}

	for _, prefix := range deniedPrefixes {
		if resolved == prefix || strings.HasPrefix(resolved, prefix+string(filepath.Separator)) {
			return true, nil
		}
	}

	return false, nil
}

// IsOutsideCWD checks whether a path resolves to a location outside the current
// working directory. Returns (true, nil) if outside, (false, nil) if inside.
// On resolution failure, returns (true, err) — fail closed.
func IsOutsideCWD(path string) (bool, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return true, err
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return true, err
	}

	rel, err := filepath.Rel(cwd, absPath)
	if err != nil {
		return true, err
	}

	if strings.HasPrefix(rel, "..") {
		return true, nil
	}

	return false, nil
}

// resolveWithAncestor walks up from absPath until it finds an existing ancestor,
// resolves symlinks on that ancestor, and appends the remaining path components.
func resolveWithAncestor(absPath string) (string, error) {
	// Collect path components from bottom up until we find an existing ancestor.
	remaining := absPath
	var tail []string

	for {
		tail = append([]string{filepath.Base(remaining)}, tail...)
		parent := filepath.Dir(remaining)

		// Reached filesystem root without finding existing dir.
		if parent == remaining {
			return "", fmt.Errorf("no existing ancestor found for %s", absPath)
		}

		resolved, err := filepath.EvalSymlinks(parent)
		if err == nil {
			// Found an existing ancestor -- reconstruct the full resolved path.
			for _, component := range tail {
				resolved = filepath.Join(resolved, component)
			}
			return resolved, nil
		}

		remaining = parent
	}
}
