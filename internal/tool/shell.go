package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"syscall"
	"time"

	"github.com/ollama/ollama/api"
)

const maxOutput = 4096

// ShellResult holds the output of a shell command execution.
type ShellResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
	TimedOut bool   `json:"timed_out,omitempty"`
}

// ToJSON marshals the result to JSON, truncating stdout and stderr if needed.
func (r *ShellResult) ToJSON() string {
	// Work on a copy to avoid mutating the receiver.
	out := *r
	if len(out.Stdout) > maxOutput {
		out.Stdout = out.Stdout[:maxOutput] + "\n... (truncated)"
	}
	if len(out.Stderr) > maxOutput {
		out.Stderr = out.Stderr[:maxOutput] + "\n... (truncated)"
	}
	b, _ := json.Marshal(out)
	return string(b)
}

// ShellTool executes shell commands and returns structured results.
type ShellTool struct {
	timeout  time.Duration
	approver ApproverFunc
}

// NewShellTool creates a ShellTool with the given timeout and optional approver.
// If approver is nil, all dangerous commands are denied.
func NewShellTool(timeout time.Duration, approver ApproverFunc) *ShellTool {
	return &ShellTool{
		timeout:  timeout,
		approver: approver,
	}
}

// Name returns the tool identifier used for dispatch.
func (s *ShellTool) Name() string {
	return "shell_exec"
}

// Definition returns the Ollama API tool definition for ChatRequest.Tools.
func (s *ShellTool) Definition() api.Tool {
	props := api.NewToolPropertiesMap()
	props.Set("command", api.ToolProperty{
		Type:        api.PropertyType{"string"},
		Description: "The shell command to execute",
	})

	return api.Tool{
		Type: "function",
		Function: api.ToolFunction{
			Name:        "shell_exec",
			Description: "Execute a shell command and return stdout, stderr, and exit code. Use this to run programs, inspect the filesystem, or perform system operations.",
			Parameters: api.ToolFunctionParameters{
				Type:       "object",
				Required:   []string{"command"},
				Properties: props,
			},
		},
	}
}

// Execute runs the shell command from the tool call arguments.
func (s *ShellTool) Execute(ctx context.Context, args api.ToolCallFunctionArguments) (string, error) {
	cmdVal, ok := args.Get("command")
	if !ok {
		return "", fmt.Errorf("missing required argument: command")
	}
	command, ok := cmdVal.(string)
	if !ok {
		return "", fmt.Errorf("missing required argument: command")
	}

	if IsDangerous(command) {
		if s.approver == nil {
			return "", fmt.Errorf("dangerous command denied: no approver configured")
		}
		if !s.approver(command) {
			return "", fmt.Errorf("dangerous command denied by user: %s", command)
		}
	}

	result, err := executeShell(ctx, command, s.timeout)
	if err != nil {
		return "", err
	}
	return result.ToJSON(), nil
}

// executeShell runs a command via /bin/sh -c with timeout enforcement.
func executeShell(ctx context.Context, command string, timeout time.Duration) (*ShellResult, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	cmd.WaitDelay = 5 * time.Second
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := &ShellResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.TimedOut = true
			result.ExitCode = -1
			return result, nil
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			return result, nil
		}
		return nil, fmt.Errorf("command execution failed: %w", err)
	}

	return result, nil
}
