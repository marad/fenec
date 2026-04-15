package profile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFullProfile(t *testing.T) {
	input := []byte("+++\nmodel = \"ollama/gemma4\"\ndescription = \"Coding assistant\"\n+++\nYou are a helpful coding assistant.\nFocus on Go and Python.\n")

	p, err := Parse(input)
	require.NoError(t, err)
	assert.Equal(t, "ollama/gemma4", p.Frontmatter.Model)
	assert.Equal(t, "Coding assistant", p.Frontmatter.Description)
	assert.Equal(t, "ollama", p.Provider)
	assert.Equal(t, "gemma4", p.ModelName)
	assert.Equal(t, "You are a helpful coding assistant.\nFocus on Go and Python.", p.SystemPrompt)
}

func TestParseBareModel(t *testing.T) {
	input := []byte("+++\nmodel = \"gemma4\"\n+++\nSome prompt.\n")

	p, err := Parse(input)
	require.NoError(t, err)
	assert.Equal(t, "", p.Provider)
	assert.Equal(t, "gemma4", p.ModelName)
}

func TestParseWithDescription(t *testing.T) {
	input := []byte("+++\nmodel = \"ollama/gemma4\"\ndescription = \"Coding assistant\"\n+++\nPrompt text.\n")

	p, err := Parse(input)
	require.NoError(t, err)
	assert.Equal(t, "Coding assistant", p.Frontmatter.Description)
}

func TestParseEmptyModel(t *testing.T) {
	input := []byte("+++\nmodel = \"\"\n+++\nPrompt.\n")

	p, err := Parse(input)
	require.NoError(t, err)
	assert.Equal(t, "", p.Provider)
	assert.Equal(t, "", p.ModelName)
}

func TestParseMissingFrontmatter(t *testing.T) {
	input := []byte("Just some text without frontmatter.\n")

	_, err := Parse(input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing frontmatter")
}

func TestParseSingleDelimiterOnly(t *testing.T) {
	input := []byte("+++\nmodel = \"test\"\n")

	_, err := Parse(input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing frontmatter")
}

func TestParseMalformedTOML(t *testing.T) {
	input := []byte("+++\nthis is = = = not valid toml\n+++\nBody.\n")

	_, err := Parse(input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing frontmatter")
}

func TestParseEmptyBody(t *testing.T) {
	input := []byte("+++\nmodel = \"ollama/gemma4\"\n+++\n")

	p, err := Parse(input)
	require.NoError(t, err)
	assert.Equal(t, "", p.SystemPrompt)
}

func TestParseBodyWhitespaceTrimmed(t *testing.T) {
	input := []byte("+++\nmodel = \"test\"\n+++\n\n  Hello world  \n\n")

	p, err := Parse(input)
	require.NoError(t, err)
	assert.Equal(t, "Hello world", p.SystemPrompt)
}

func TestParseUnknownFieldsIgnored(t *testing.T) {
	input := []byte("+++\nmodel = \"test\"\nunknown_field = \"whatever\"\nanother = 42\n+++\nBody.\n")

	p, err := Parse(input)
	require.NoError(t, err)
	assert.Equal(t, "test", p.ModelName)
}

func TestParseModelSplitVariants(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		provider string
		modelN   string
	}{
		{"provider/model", "ollama/gemma4", "ollama", "gemma4"},
		{"bare model", "gemma4", "", "gemma4"},
		{"empty", "", "", ""},
		{"copilot/gpt-4o", "copilot/gpt-4o", "copilot", "gpt-4o"},
		{"multiple slashes", "a/b/c", "a", "b/c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := []byte("+++\nmodel = \"" + tt.model + "\"\n+++\nBody.\n")
			p, err := Parse(input)
			require.NoError(t, err)
			assert.Equal(t, tt.provider, p.Provider)
			assert.Equal(t, tt.modelN, p.ModelName)
		})
	}
}

// --- Test helpers ---

func writeTestProfile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name+".md"), []byte(content), 0644))
}

const coderProfile = "+++\nmodel = \"ollama/gemma4\"\ndescription = \"Coding assistant\"\n+++\nYou are a coding assistant. Focus on clean Go code.\n"
const writerProfile = "+++\nmodel = \"copilot/gpt-4o\"\n+++\nYou are a creative writing assistant.\n"

// --- Load tests ---

func TestLoadExistingProfile(t *testing.T) {
	dir := t.TempDir()
	writeTestProfile(t, dir, "coder", coderProfile)

	p, err := Load(dir, "coder")
	require.NoError(t, err)
	assert.Equal(t, "coder", p.Name)
	assert.Equal(t, "ollama/gemma4", p.Frontmatter.Model)
	assert.Equal(t, "ollama", p.Provider)
	assert.Equal(t, "gemma4", p.ModelName)
	assert.Equal(t, "Coding assistant", p.Frontmatter.Description)
	assert.Contains(t, p.SystemPrompt, "coding assistant")
}

func TestLoadNonExistentProfile(t *testing.T) {
	dir := t.TempDir()

	_, err := Load(dir, "coder")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "coder.md")
}

func TestLoadPathTraversalDotDot(t *testing.T) {
	dir := t.TempDir()

	_, err := Load(dir, "../etc/passwd")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid profile name")
}

func TestLoadPathTraversalSlash(t *testing.T) {
	dir := t.TempDir()

	_, err := Load(dir, "foo/bar")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid profile name")
}

func TestLoadPathTraversalBackslash(t *testing.T) {
	dir := t.TempDir()

	_, err := Load(dir, "foo\\bar")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid profile name")
}

func TestLoadPathTraversalDot(t *testing.T) {
	dir := t.TempDir()

	_, err := Load(dir, ".hidden")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid profile name")
}

// --- List tests ---

func TestListWithProfiles(t *testing.T) {
	dir := t.TempDir()
	writeTestProfile(t, dir, "coder", coderProfile)
	writeTestProfile(t, dir, "writer", writerProfile)

	summaries, err := List(dir)
	require.NoError(t, err)
	require.Len(t, summaries, 2)

	// Should be sorted by name.
	assert.Equal(t, "coder", summaries[0].Name)
	assert.Equal(t, "ollama/gemma4", summaries[0].Model)
	assert.Equal(t, "writer", summaries[1].Name)
	assert.Equal(t, "copilot/gpt-4o", summaries[1].Model)
}

func TestListEmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	summaries, err := List(dir)
	require.NoError(t, err)
	assert.Empty(t, summaries)
}

func TestListNonExistentDirectory(t *testing.T) {
	summaries, err := List("/nonexistent/path/profiles")
	require.NoError(t, err)
	assert.Empty(t, summaries)
}

func TestListSkipsNonMdFiles(t *testing.T) {
	dir := t.TempDir()
	writeTestProfile(t, dir, "coder", coderProfile)
	// Write a non-.md file.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.txt"), []byte("ignore me"), 0644))

	summaries, err := List(dir)
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	assert.Equal(t, "coder", summaries[0].Name)
}
