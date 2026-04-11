package tool

import (
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
	// resolve the parent directory and append the base name.
	resolved, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		parentResolved, parentErr := filepath.EvalSymlinks(filepath.Dir(absPath))
		if parentErr != nil {
			return true, parentErr
		}
		resolved = filepath.Join(parentResolved, filepath.Base(absPath))
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
