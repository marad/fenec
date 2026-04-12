package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamMetricsJSONRoundTrip(t *testing.T) {
	m := StreamMetrics{
		PromptEvalCount: 100,
		EvalCount:       50,
	}

	data, err := json.Marshal(m)
	require.NoError(t, err)

	var got StreamMetrics
	err = json.Unmarshal(data, &got)
	require.NoError(t, err)

	assert.Equal(t, 100, got.PromptEvalCount)
	assert.Equal(t, 50, got.EvalCount)
}

func TestStreamMetricsJSONOmitsZeroValues(t *testing.T) {
	m := StreamMetrics{}

	data, err := json.Marshal(m)
	require.NoError(t, err)

	jsonStr := string(data)
	assert.NotContains(t, jsonStr, "prompt_eval_count")
	assert.NotContains(t, jsonStr, "eval_count")
	assert.Equal(t, `{}`, jsonStr)
}

func TestStreamMetricsJSONPartialValues(t *testing.T) {
	m := StreamMetrics{
		EvalCount: 42,
	}

	data, err := json.Marshal(m)
	require.NoError(t, err)

	jsonStr := string(data)
	assert.NotContains(t, jsonStr, "prompt_eval_count")
	assert.Contains(t, jsonStr, `"eval_count":42`)
}
