package chat

// ContextTracker monitors token usage and manages context window truncation.
type ContextTracker struct {
	maxTokens      int
	threshold      float64
	lastPromptEval int
	lastEval       int
}

// NewContextTracker creates a tracker for the given context window size.
// threshold is the fraction (0.0-1.0) at which truncation triggers (e.g., 0.85 = 85%).
func NewContextTracker(maxTokens int, threshold float64) *ContextTracker {
	return &ContextTracker{
		maxTokens: maxTokens,
		threshold: threshold,
	}
}

// Update records the latest token counts from Ollama Metrics.
func (ct *ContextTracker) Update(promptEvalCount, evalCount int) {
	ct.lastPromptEval = promptEvalCount
	ct.lastEval = evalCount
}

// TokenUsage returns the current total token usage (prompt + completion).
func (ct *ContextTracker) TokenUsage() int {
	return ct.lastPromptEval + ct.lastEval
}

// ShouldTruncate returns true if token usage exceeds the threshold.
func (ct *ContextTracker) ShouldTruncate() bool {
	return ct.TokenUsage() >= int(float64(ct.maxTokens)*ct.threshold)
}

// Available returns the maximum context window size in tokens.
func (ct *ContextTracker) Available() int {
	return ct.maxTokens
}

// Threshold returns the configured truncation threshold.
func (ct *ContextTracker) Threshold() float64 {
	return ct.threshold
}

// TruncateOldest removes the oldest non-system messages from the conversation
// until token usage estimate drops below the threshold.
// Returns the number of messages removed.
//
// Strategy: remove messages from the front (after system messages) in pairs
// (user + assistant). Estimate token reduction proportionally based on
// message count reduction. The actual token count will be corrected by the
// next PromptEvalCount from Ollama.
func (ct *ContextTracker) TruncateOldest(conv *Conversation) int {
	if !ct.ShouldTruncate() {
		return 0
	}

	limit := int(float64(ct.maxTokens) * ct.threshold)
	currentTokens := ct.TokenUsage()
	removed := 0

	// Find first non-system message
	start := 0
	for start < len(conv.Messages) && conv.Messages[start].Role == "system" {
		start++
	}

	// Remove messages from the front (oldest first)
	for currentTokens > limit && start < len(conv.Messages) {
		// Try to remove a pair (user + assistant) if both exist
		removeCount := 1
		if start+1 < len(conv.Messages) {
			removeCount = 2
		}

		totalBefore := len(conv.Messages) + removeCount // message count before removal (for ratio)
		conv.Messages = append(conv.Messages[:start], conv.Messages[start+removeCount:]...)
		removed += removeCount

		// Estimate reduction proportionally
		if totalBefore > 0 {
			currentTokens = currentTokens * len(conv.Messages) / totalBefore
		}
	}

	return removed
}
