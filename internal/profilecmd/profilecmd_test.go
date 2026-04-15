package profilecmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Test helpers ---

func writeTestProfile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name+".md"), []byte(content), 0644))
}

const testCoderProfile = "+++\nmodel = \"ollama/gemma4\"\ndescription = \"Coding assistant\"\n+++\nYou are a coding assistant.\n"
const testWriterProfile = "+++\nmodel = \"copilot/gpt-4o\"\n+++\nYou are a writer.\n"
const testMinimalProfile = "+++\nmodel = \"\"\n+++\nMinimal.\n"

// --- List tests ---

func TestRunListWithProfiles(t *testing.T) {
	dir := t.TempDir()
	writeTestProfile(t, dir, "coder", testCoderProfile)
	writeTestProfile(t, dir, "writer", testWriterProfile)

	var buf bytes.Buffer
	err := runList(&buf, dir)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "NAME")
	assert.Contains(t, output, "MODEL")
	assert.Contains(t, output, "coder")
	assert.Contains(t, output, "ollama/gemma4")
	assert.Contains(t, output, "writer")
	assert.Contains(t, output, "copilot/gpt-4o")
}

func TestRunListEmpty(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	err := runList(&buf, dir)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No profiles found.")
}

func TestRunListEmptyModel(t *testing.T) {
	dir := t.TempDir()
	writeTestProfile(t, dir, "minimal", testMinimalProfile)
	var buf bytes.Buffer
	err := runList(&buf, dir)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "(default)")
}

func TestRunListNonExistentDir(t *testing.T) {
	var buf bytes.Buffer
	err := runList(&buf, "/nonexistent/path/profiles")
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No profiles found.")
}

// --- Create tests ---

func TestRunCreateNewProfile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EDITOR", "true")
	err := doCreate(dir, "myprofile")
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "myprofile.md"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "+++")
	assert.Contains(t, string(data), `model = ""`)
}

func TestRunCreateAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	writeTestProfile(t, dir, "existing", testCoderProfile)
	t.Setenv("EDITOR", "true")
	err := doCreate(dir, "existing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestRunCreateInvalidNameSlash(t *testing.T) {
	dir := t.TempDir()
	err := doCreate(dir, "foo/bar")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid profile name")
}

func TestRunCreateInvalidNameDot(t *testing.T) {
	dir := t.TempDir()
	err := doCreate(dir, ".hidden")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid profile name")
}

func TestRunCreateInvalidNameBackslash(t *testing.T) {
	dir := t.TempDir()
	err := doCreate(dir, "foo\\bar")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid profile name")
}

func TestRunCreateCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "profiles")
	t.Setenv("EDITOR", "true")
	err := doCreate(dir, "newprofile")
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(dir, "newprofile.md"))
}

// --- Edit tests ---

func TestRunEditNonExistent(t *testing.T) {
	dir := t.TempDir()
	err := doEdit(dir, "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRunEditInvalidName(t *testing.T) {
	dir := t.TempDir()
	err := doEdit(dir, "../etc")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid profile name")
}

func TestRunEditExistingProfile(t *testing.T) {
	dir := t.TempDir()
	writeTestProfile(t, dir, "coder", testCoderProfile)
	t.Setenv("EDITOR", "true")
	err := doEdit(dir, "coder")
	require.NoError(t, err)
}

// --- Editor tests ---

func TestGetEditorDefault(t *testing.T) {
	t.Setenv("EDITOR", "")
	assert.Equal(t, "vi", getEditor())
}

func TestGetEditorFromEnv(t *testing.T) {
	t.Setenv("EDITOR", "nano")
	assert.Equal(t, "nano", getEditor())
}
