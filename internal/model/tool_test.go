package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPropertyTypeMarshalSingleString(t *testing.T) {
	pt := PropertyType{"string"}

	data, err := json.Marshal(pt)
	require.NoError(t, err)

	// Should marshal as bare string, not array
	assert.Equal(t, `"string"`, string(data))
}

func TestPropertyTypeMarshalMultipleStrings(t *testing.T) {
	pt := PropertyType{"string", "null"}

	data, err := json.Marshal(pt)
	require.NoError(t, err)

	// Should marshal as array
	assert.Equal(t, `["string","null"]`, string(data))
}

func TestPropertyTypeUnmarshalBareString(t *testing.T) {
	var pt PropertyType
	err := json.Unmarshal([]byte(`"string"`), &pt)
	require.NoError(t, err)

	assert.Equal(t, PropertyType{"string"}, pt)
}

func TestPropertyTypeUnmarshalArray(t *testing.T) {
	var pt PropertyType
	err := json.Unmarshal([]byte(`["string","null"]`), &pt)
	require.NoError(t, err)

	assert.Equal(t, PropertyType{"string", "null"}, pt)
}

func TestToolDefinitionJSONRoundTrip(t *testing.T) {
	td := ToolDefinition{
		Type: "function",
		Function: ToolFunction{
			Name:        "read_file",
			Description: "Read a file",
			Parameters: ToolFunctionParameters{
				Type:     "object",
				Required: []string{"path"},
				Properties: map[string]ToolProperty{
					"path": {
						Type:        PropertyType{"string"},
						Description: "File path to read",
					},
				},
			},
		},
	}

	data, err := json.Marshal(td)
	require.NoError(t, err)

	var got ToolDefinition
	err = json.Unmarshal(data, &got)
	require.NoError(t, err)

	assert.Equal(t, "function", got.Type)
	assert.Equal(t, "read_file", got.Function.Name)
	assert.Equal(t, "Read a file", got.Function.Description)
	assert.Equal(t, "object", got.Function.Parameters.Type)
	assert.Equal(t, []string{"path"}, got.Function.Parameters.Required)

	pathProp, ok := got.Function.Parameters.Properties["path"]
	require.True(t, ok)
	assert.Equal(t, PropertyType{"string"}, pathProp.Type)
	assert.Equal(t, "File path to read", pathProp.Description)
}

func TestToolFunctionParametersJSONSchema(t *testing.T) {
	params := ToolFunctionParameters{
		Type:     "object",
		Required: []string{"query"},
		Properties: map[string]ToolProperty{
			"query": {
				Type:        PropertyType{"string"},
				Description: "Search query",
			},
			"limit": {
				Type:        PropertyType{"integer"},
				Description: "Max results",
			},
		},
	}

	data, err := json.Marshal(params)
	require.NoError(t, err)

	// Verify it's valid JSON
	var raw map[string]any
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	assert.Equal(t, "object", raw["type"])

	props, ok := raw["properties"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, props, "query")
	assert.Contains(t, props, "limit")
}

func TestToolPropertyWithEnum(t *testing.T) {
	prop := ToolProperty{
		Type:        PropertyType{"string"},
		Description: "Output format",
		Enum:        []any{"json", "text", "csv"},
	}

	data, err := json.Marshal(prop)
	require.NoError(t, err)

	var got ToolProperty
	err = json.Unmarshal(data, &got)
	require.NoError(t, err)

	assert.Equal(t, PropertyType{"string"}, got.Type)
	assert.Len(t, got.Enum, 3)
}

func TestToolDefinitionJSONFormat(t *testing.T) {
	td := ToolDefinition{
		Type: "function",
		Function: ToolFunction{
			Name:        "test_tool",
			Description: "A test tool",
			Parameters: ToolFunctionParameters{
				Type:     "object",
				Required: []string{"arg1"},
				Properties: map[string]ToolProperty{
					"arg1": {
						Type:        PropertyType{"string"},
						Description: "First argument",
					},
				},
			},
		},
	}

	data, err := json.Marshal(td)
	require.NoError(t, err)

	jsonStr := string(data)
	// Verify JSON schema structure
	assert.Contains(t, jsonStr, `"type":"function"`)
	assert.Contains(t, jsonStr, `"function":{`)
	assert.Contains(t, jsonStr, `"name":"test_tool"`)
	assert.Contains(t, jsonStr, `"parameters":{`)
}
