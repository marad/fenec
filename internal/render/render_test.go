package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatPrompt(t *testing.T) {
	prompt := FormatPrompt("gemma4")
	assert.Contains(t, prompt, "gemma4")
	assert.Contains(t, prompt, "]>")
}

func TestFormatBanner(t *testing.T) {
	banner := FormatBanner("v0.1")
	assert.Contains(t, banner, "fenec")
	assert.Contains(t, banner, "v0.1")
	assert.Contains(t, banner, "/help")
}

func TestFormatError(t *testing.T) {
	errMsg := FormatError("something went wrong")
	assert.Contains(t, errMsg, "Error:")
	assert.Contains(t, errMsg, "something went wrong")
}
