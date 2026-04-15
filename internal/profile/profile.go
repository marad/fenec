package profile

import (
	"fmt"
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
