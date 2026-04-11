package render

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/briandowns/spinner"
)

// Spinner wraps the braindowns/spinner for the thinking indicator.
type Spinner struct {
	s       *spinner.Spinner
	w       io.Writer
	stopped sync.Once
}

// NewSpinner creates a thinking indicator that writes to the given writer.
// Use rl.Stdout() as the writer to avoid readline prompt corruption.
// Per D-05: shown between user pressing Enter and first token arriving.
func NewSpinner(w io.Writer) *Spinner {
	s := spinner.New(spinner.CharSets[11], 80*time.Millisecond)
	s.Suffix = " Thinking..."
	s.Writer = w

	return &Spinner{
		s: s,
		w: w,
	}
}

// Start begins the spinner animation.
func (sp *Spinner) Start() {
	sp.s.Start()
}

// Stop halts the spinner and clears its line. Safe to call multiple times.
func (sp *Spinner) Stop() {
	sp.stopped.Do(func() {
		sp.s.Stop()
		fmt.Fprint(sp.w, "\r\033[K")
	})
}
