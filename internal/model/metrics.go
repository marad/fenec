package model

// StreamMetrics holds the subset of model performance metrics
// that Fenec tracks from streaming responses.
type StreamMetrics struct {
	PromptEvalCount int `json:"prompt_eval_count,omitempty"`
	EvalCount       int `json:"eval_count,omitempty"`
}
