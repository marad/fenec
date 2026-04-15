package profilecmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/marad/fenec/internal/config"
	"github.com/marad/fenec/internal/profile"
	"github.com/marad/fenec/internal/render"
)

const profileTemplate = `+++
model = ""
description = ""
+++

Write your system prompt here.
`

// Run is the entry point for `fenec profile` subcommands.
// It receives os.Args[2:] (everything after "fenec profile").
func Run(args []string) {
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}
	switch args[0] {
	case "list":
		dir, err := config.ProfilesDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, render.FormatError(
				fmt.Sprintf("resolving profiles directory: %v", err)))
			os.Exit(1)
		}
		if err := runList(os.Stdout, dir); err != nil {
			fmt.Fprintln(os.Stderr, render.FormatError(err.Error()))
			os.Exit(1)
		}
	case "create":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, render.FormatError("missing profile name"))
			fmt.Fprintln(os.Stderr, "Usage: fenec profile create <name>")
			os.Exit(1)
		}
		dir, err := config.ProfilesDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, render.FormatError(
				fmt.Sprintf("resolving profiles directory: %v", err)))
			os.Exit(1)
		}
		if err := doCreate(dir, args[1]); err != nil {
			fmt.Fprintln(os.Stderr, render.FormatError(err.Error()))
			os.Exit(1)
		}
	case "edit":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, render.FormatError("missing profile name"))
			fmt.Fprintln(os.Stderr, "Usage: fenec profile edit <name>")
			os.Exit(1)
		}
		dir, err := config.ProfilesDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, render.FormatError(
				fmt.Sprintf("resolving profiles directory: %v", err)))
			os.Exit(1)
		}
		if err := doEdit(dir, args[1]); err != nil {
			fmt.Fprintln(os.Stderr, render.FormatError(err.Error()))
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown profile subcommand: %s\n", args[0])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage: fenec profile <command>

Commands:
  list              List available profiles
  create <name>     Create a new profile and open in $EDITOR
  edit <name>       Edit an existing profile in $EDITOR`)
}

// runList prints a tabwriter-aligned table of available profiles.
// Accepts dir parameter for testability (avoids calling config.ProfilesDir in tests).
func runList(w io.Writer, dir string) error {
	summaries, err := profile.List(dir)
	if err != nil {
		return err
	}
	if len(summaries) == 0 {
		fmt.Fprintln(w, "No profiles found.")
		fmt.Fprintf(w, "Create one with: fenec profile create <name>\n")
		return nil
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tMODEL")
	for _, s := range summaries {
		model := s.Model
		if model == "" {
			model = "(default)"
		}
		fmt.Fprintf(tw, "%s\t%s\n", s.Name, model)
	}
	return tw.Flush()
}

// doCreate creates a new profile file and opens it in the user's editor.
// Accepts dir parameter for testability.
func doCreate(dir, name string) error {
	if strings.ContainsAny(name, "/\\.") {
		return fmt.Errorf("invalid profile name: %q", name)
	}
	// Ensure profiles directory exists.
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating profiles directory: %w", err)
	}
	path := filepath.Join(dir, name+".md")
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("profile %q already exists — use 'fenec profile edit %s' instead", name, name)
	}
	if err := os.WriteFile(path, []byte(profileTemplate), 0644); err != nil {
		return fmt.Errorf("writing profile: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Created profile: %s\n", path)
	return openEditor(getEditor(), path)
}

// doEdit opens an existing profile file in the user's editor.
// Accepts dir parameter for testability.
func doEdit(dir, name string) error {
	if strings.ContainsAny(name, "/\\.") {
		return fmt.Errorf("invalid profile name: %q", name)
	}
	path := filepath.Join(dir, name+".md")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("profile %q not found — use 'fenec profile create %s' to create it", name, name)
	}
	return openEditor(getEditor(), path)
}

// getEditor returns the user's preferred editor from $EDITOR, falling back to "vi".
func getEditor() string {
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}
	return "vi"
}

// openEditor launches the editor with the given file path.
// Handles $EDITOR values with arguments (e.g., "code --wait").
func openEditor(editor, path string) error {
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		return fmt.Errorf("empty editor command")
	}
	args := append(parts[1:], path)
	cmd := exec.Command(parts[0], args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
