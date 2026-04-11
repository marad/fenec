package repl

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// PageOutput writes content to w, pausing every pageHeight lines
// with a "--more--" prompt. User presses Enter to continue or q to stop.
// Per D-08: Auto-page long responses that exceed terminal height.
// reader is used to read user input for paging (typically os.Stdin).
func PageOutput(w io.Writer, content string, pageHeight int, reader io.Reader) {
	lines := strings.Split(content, "\n")

	// If content fits on screen, write all at once.
	if len(lines) <= pageHeight {
		fmt.Fprint(w, content)
		return
	}

	written := 0
	for written < len(lines) {
		// Write one page worth of lines (pageHeight - 1 to leave room for prompt).
		end := written + pageHeight - 1
		if end > len(lines) {
			end = len(lines)
		}

		chunk := strings.Join(lines[written:end], "\n")
		fmt.Fprint(w, chunk)
		if end < len(lines) {
			fmt.Fprint(w, "\n")
		}
		written = end

		// If there are more lines, show the pager prompt.
		if written < len(lines) {
			fmt.Fprint(w, "--more-- (Enter: next page, q: quit)")

			// Read a single byte for input.
			buf := make([]byte, 1)
			_, err := reader.Read(buf)
			if err != nil {
				break
			}

			// Clear the pager prompt line.
			fmt.Fprint(w, "\r\033[K")

			if buf[0] == 'q' || buf[0] == 'Q' {
				break
			}
			// Any other key (including Enter) continues.
		}
	}
}

// TerminalHeight returns the terminal height in rows.
// Falls back to 24 if detection fails.
func TerminalHeight() int {
	_, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || h <= 0 {
		return 24
	}
	return h
}

// TerminalWidth returns the terminal width in columns.
// Falls back to 80 if detection fails.
func TerminalWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return 80
	}
	return w
}
