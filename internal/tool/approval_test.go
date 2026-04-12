package tool

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestYoloApproverAutoApproves verifies that a yolo-mode approver
// auto-approves all dangerous commands.
func TestYoloApproverAutoApproves(t *testing.T) {
	var log bytes.Buffer
	approver := func(command string) bool {
		fmt.Fprintf(&log, "[yolo] auto-approved: %s\n", command)
		return true
	}

	st := NewShellTool(10*time.Second, approver)

	// A dangerous command (rm) should be auto-approved and execute.
	result, err := st.Execute(context.Background(), makeArgs("echo dangerous-rm && rm /tmp/nonexistent_yolo_test 2>/dev/null || true"))
	require.NoError(t, err)
	assert.Contains(t, result, "dangerous-rm")
	assert.Contains(t, log.String(), "[yolo] auto-approved")
}

// TestNonInteractiveApproverDenies verifies that a non-interactive approver
// auto-denies all dangerous commands with an informative message.
func TestNonInteractiveApproverDenies(t *testing.T) {
	var log bytes.Buffer
	approver := func(command string) bool {
		fmt.Fprintf(&log, "[denied] %s — non-interactive mode. Use --yolo to auto-approve.\n", command)
		return false
	}

	st := NewShellTool(10*time.Second, approver)

	// A dangerous command should be denied.
	_, err := st.Execute(context.Background(), makeArgs("rm /tmp/nonexistent"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "denied")
	assert.Contains(t, log.String(), "non-interactive mode")
	assert.Contains(t, log.String(), "--yolo")
}

// TestNonInteractiveApproverAllowsSafeCommands verifies that safe commands
// still execute without approval in non-interactive mode.
func TestNonInteractiveApproverAllowsSafeCommands(t *testing.T) {
	// The approver should never be called for safe commands.
	approverCalled := false
	approver := func(command string) bool {
		approverCalled = true
		return false
	}

	st := NewShellTool(10*time.Second, approver)

	result, err := st.Execute(context.Background(), makeArgs("echo safe"))
	require.NoError(t, err)
	assert.Contains(t, result, "safe")
	assert.False(t, approverCalled, "approver should not be called for safe commands")
}

// TestYoloApproverWithWriteTool verifies yolo mode works with write tool
// approval for out-of-CWD paths.
func TestYoloApproverWithWriteTool(t *testing.T) {
	var log bytes.Buffer
	approver := func(desc string) bool {
		fmt.Fprintf(&log, "[yolo] auto-approved: %s\n", desc)
		return true
	}

	wt := NewWriteFileTool(approver)

	// Attempt to write to a temp location outside CWD.
	tmpFile := t.TempDir() + "/yolo-test.txt"
	args := writeArgs(tmpFile, "yolo content")
	result, err := wt.Execute(context.Background(), args)
	require.NoError(t, err)
	assert.Contains(t, result, "ok")
	assert.Contains(t, log.String(), "[yolo] auto-approved")
}

// TestNonInteractiveApproverWithWriteTool verifies non-interactive mode
// denies out-of-CWD writes.
func TestNonInteractiveApproverWithWriteTool(t *testing.T) {
	var log bytes.Buffer
	approver := func(desc string) bool {
		fmt.Fprintf(&log, "[denied] %s — non-interactive mode.\n", desc)
		return false
	}

	wt := NewWriteFileTool(approver)

	// Attempt to write outside CWD should be denied.
	tmpFile := t.TempDir() + "/denied-test.txt"
	args := writeArgs(tmpFile, "denied content")
	_, err := wt.Execute(context.Background(), args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "denied")
}

// TestNilApproverFallback verifies that the closure pattern used in main.go
// (where approver starts nil) correctly denies dangerous commands until wired.
func TestNilApproverFallback(t *testing.T) {
	var approver ApproverFunc

	st := NewShellTool(10*time.Second, func(command string) bool {
		if approver != nil {
			return approver(command)
		}
		return false
	})

	// Before wiring, dangerous commands should be denied.
	_, err := st.Execute(context.Background(), makeArgs("rm /tmp/nonexistent"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "denied")

	// After wiring yolo approver, dangerous commands should pass.
	approver = func(_ string) bool { return true }
	result, err := st.Execute(context.Background(), makeArgs("rm /tmp/nonexistent_fenec_nil_test 2>/dev/null || echo removed"))
	require.NoError(t, err)
	assert.Contains(t, result, "removed")
}
