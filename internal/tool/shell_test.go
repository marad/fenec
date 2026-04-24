package tool

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeArgs(command string) map[string]any {
	return map[string]any{"command": command}
}

func TestShellExecEcho(t *testing.T) {
	st := NewShellTool(10*time.Second, nil)
	result, err := st.Execute(context.Background(), makeArgs("echo hello"))
	require.NoError(t, err)

	var sr ShellResult
	require.NoError(t, json.Unmarshal([]byte(result), &sr))
	assert.Contains(t, sr.Stdout, "hello")
	assert.Equal(t, 0, sr.ExitCode)
}

func TestShellExecStderr(t *testing.T) {
	st := NewShellTool(10*time.Second, nil)
	result, err := st.Execute(context.Background(), makeArgs("echo err >&2"))
	require.NoError(t, err)

	var sr ShellResult
	require.NoError(t, json.Unmarshal([]byte(result), &sr))
	assert.Contains(t, sr.Stderr, "err")
}

func TestShellExecExitCode(t *testing.T) {
	st := NewShellTool(10*time.Second, nil)
	result, err := st.Execute(context.Background(), makeArgs("exit 42"))
	require.NoError(t, err)

	var sr ShellResult
	require.NoError(t, json.Unmarshal([]byte(result), &sr))
	assert.Equal(t, 42, sr.ExitCode)
}

func TestShellExecTimeout(t *testing.T) {
	st := NewShellTool(100*time.Millisecond, nil)
	result, err := st.Execute(context.Background(), makeArgs("sleep 10"))
	require.NoError(t, err)

	var sr ShellResult
	require.NoError(t, json.Unmarshal([]byte(result), &sr))
	assert.True(t, sr.TimedOut)
}

func TestShellExecDangerousApproved(t *testing.T) {
	approver := func(_ string) bool { return true }
	st := NewShellTool(10*time.Second, approver)
	result, err := st.Execute(context.Background(), makeArgs("rm /tmp/nonexistent_fenec_test_file"))
	require.NoError(t, err)

	var sr ShellResult
	require.NoError(t, json.Unmarshal([]byte(result), &sr))
	// rm of nonexistent file returns non-zero exit, but it should have run
	assert.NotNil(t, sr)
}

func TestShellExecDangerousDenied(t *testing.T) {
	approver := func(_ string) bool { return false }
	st := NewShellTool(10*time.Second, approver)
	_, err := st.Execute(context.Background(), makeArgs("rm /tmp/nonexistent"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "denied")
}

func TestShellExecDangerousNoApprover(t *testing.T) {
	st := NewShellTool(10*time.Second, nil)
	_, err := st.Execute(context.Background(), makeArgs("rm /tmp/nonexistent"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "denied")
	assert.Contains(t, err.Error(), "no approver configured")
}

func TestShellToolDefinition(t *testing.T) {
	st := NewShellTool(10*time.Second, nil)
	def := st.Definition()
	assert.Equal(t, "function", def.Type)
	assert.Equal(t, "shell_exec", def.Function.Name)
	assert.Contains(t, def.Function.Parameters.Required, "command")
}

func TestShellToolName(t *testing.T) {
	st := NewShellTool(10*time.Second, nil)
	assert.Equal(t, "shell_exec", st.Name())
}

func TestShellResultToJSON(t *testing.T) {
	sr := ShellResult{
		Stdout:   "hello world",
		Stderr:   "some warning",
		ExitCode: 0,
	}
	jsonStr := sr.ToJSON()
	assert.Contains(t, jsonStr, `"stdout"`)
	assert.Contains(t, jsonStr, `"stderr"`)
	assert.Contains(t, jsonStr, `"exit_code"`)

	var parsed ShellResult
	require.NoError(t, json.Unmarshal([]byte(jsonStr), &parsed))
	assert.Equal(t, "hello world", parsed.Stdout)
	assert.Equal(t, "some warning", parsed.Stderr)
	assert.Equal(t, 0, parsed.ExitCode)
}

func TestShellResultTruncation(t *testing.T) {
	longOutput := strings.Repeat("x", 5000)
	sr := ShellResult{
		Stdout:   longOutput,
		ExitCode: 0,
	}
	jsonStr := sr.ToJSON()

	var parsed ShellResult
	require.NoError(t, json.Unmarshal([]byte(jsonStr), &parsed))
	assert.LessOrEqual(t, len(parsed.Stdout), 4096+len("\n... (truncated)"))
	assert.True(t, strings.HasSuffix(parsed.Stdout, "... (truncated)"))
}

func TestShellExecMissingCommand(t *testing.T) {
	st := NewShellTool(10*time.Second, nil)
	args := map[string]any{}
	_, err := st.Execute(context.Background(), args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required argument: command")
}

// === Tests for rune-safe truncation (L-2) ===

func TestTruncateUTF8_ascii(t *testing.T) {
	result := truncateUTF8("hello world", 5)
	assert.Equal(t, "hello", result)
}

func TestTruncateUTF8_no_truncation_needed(t *testing.T) {
	result := truncateUTF8("short", 100)
	assert.Equal(t, "short", result)
}

func TestTruncateUTF8_mid_rune(t *testing.T) {
	// "ö" is 2 bytes in UTF-8 (0xC3 0xB6). Cutting after 1st byte is invalid.
	s := "aöb" // 4 bytes: 'a'(1) + 'ö'(2) + 'b'(1)
	result := truncateUTF8(s, 2)
	// Should cut back to "a" (1 byte) since byte 2 is mid-rune
	assert.Equal(t, "a", result)
}

func TestTruncateUTF8_emoji(t *testing.T) {
	// "🎉" is 4 bytes in UTF-8. Cutting at byte 2 should produce empty or prior chars.
	s := "🎉x"
	result := truncateUTF8(s, 2)
	// Bytes 0-3 are the emoji, byte 2 is mid-rune, so truncation walks back to 0
	assert.Equal(t, "", result)
}

func TestShellResultTruncation_UTF8(t *testing.T) {
	// Build output that ends with multi-byte chars near the truncation boundary
	prefix := strings.Repeat("x", maxOutput-2)
	// Append a 3-byte UTF-8 char that would be split if truncated naively
	longOutput := prefix + "€€€" // € is 3 bytes (0xE2 0x82 0xAC)
	sr := ShellResult{Stdout: longOutput, ExitCode: 0}
	jsonStr := sr.ToJSON()

	// Must produce valid JSON (would fail if mid-rune bytes appear)
	var parsed ShellResult
	require.NoError(t, json.Unmarshal([]byte(jsonStr), &parsed))
	assert.True(t, strings.HasSuffix(parsed.Stdout, "... (truncated)"))
}
