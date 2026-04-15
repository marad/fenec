package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// Frontmatter represents the TOML frontmatter of a profile file.
type Frontmatter struct {
	Model       string `toml:"model"`
	Description string `toml:"description"`
}

// Profile represents a parsed profile with metadata and system prompt.
type Profile struct {
	Name         string      // Derived from filename (set by Load, not Parse)
	Frontmatter  Frontmatter // Parsed TOML frontmatter
	Provider     string      // Extracted provider from "provider/model" (empty if bare model)
	ModelName    string      // Extracted model name from "provider/model" or bare model
	SystemPrompt string      // Markdown body after frontmatter
}

const delimiter = "+++"

// Parse parses a profile file with +++‑delimited TOML frontmatter and a
// markdown body. It returns a Profile with the frontmatter decoded, the
// provider/model split computed, and the system prompt extracted.
func Parse(data []byte) (*Profile, error) {
	content := string(data)
	lines := strings.Split(content, "\n")

	// Find the two +++ delimiter lines.
	first := -1
	second := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == delimiter {
			if first == -1 {
				first = i
			} else {
				second = i
				break
			}
		}
	}

	if first == -1 || second == -1 {
		return nil, fmt.Errorf("missing frontmatter: no +++ delimiters found")
	}

	// Extract TOML content between delimiters.
	tomlContent := strings.Join(lines[first+1:second], "\n")

	var fm Frontmatter
	if _, err := toml.Decode(tomlContent, &fm); err != nil {
		return nil, fmt.Errorf("parsing frontmatter: %w", err)
	}

	// Extract body after second delimiter, trim whitespace.
	body := strings.Join(lines[second+1:], "\n")
	body = strings.TrimSpace(body)

	// Split model field into provider and model name.
	var provider, modelName string
	if fm.Model != "" {
		if strings.Contains(fm.Model, "/") {
			parts := strings.SplitN(fm.Model, "/", 2)
			provider = parts[0]
			modelName = parts[1]
		} else {
			modelName = fm.Model
		}
	}

	return &Profile{
		Frontmatter:  fm,
		Provider:     provider,
		ModelName:    modelName,
		SystemPrompt: body,
	}, nil
}

// ProfileSummary is a lightweight view for profile listing.
type ProfileSummary struct {
	Name  string // Derived from filename (without .md)
	Model string // Raw model field from frontmatter
}

// Load reads a profile by name from the given directory and returns the parsed
// Profile. The name must not contain path separators or dots (path traversal
// protection).
func Load(dir, name string) (*Profile, error) {
	if strings.ContainsAny(name, "/\\.") {
		return nil, fmt.Errorf("invalid profile name: %q", name)
	}

	path := filepath.Join(dir, name+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loading profile %s: %w", name+".md", err)
	}

	profile, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("loading profile %s: %w", name+".md", err)
	}

	profile.Name = name
	return profile, nil
}

// List reads all .md profile files from the given directory and returns a
// sorted slice of ProfileSummary. Returns an empty slice (not an error) if the
// directory does not exist or is empty.
func List(dir string) ([]ProfileSummary, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("listing profiles: %w", err)
	}

	var summaries []ProfileSummary
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue // skip unreadable files
		}

		profile, err := Parse(data)
		if err != nil {
			continue // skip unparseable files
		}

		name := strings.TrimSuffix(entry.Name(), ".md")
		summaries = append(summaries, ProfileSummary{
			Name:  name,
			Model: profile.Frontmatter.Model,
		})
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Name < summaries[j].Name
	})

	return summaries, nil
}
