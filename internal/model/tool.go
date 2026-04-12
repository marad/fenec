package model

import "encoding/json"

// ToolDefinition describes a tool available to the model.
// Mirrors ollama/api.Tool with identical JSON schema format.
type ToolDefinition struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction describes the function component of a tool definition.
type ToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  ToolFunctionParameters `json:"parameters"`
}

// ToolFunctionParameters describes the parameters accepted by a tool function.
// Uses a plain map for properties instead of an ordered map.
type ToolFunctionParameters struct {
	Type       string                  `json:"type"`
	Required   []string                `json:"required,omitempty"`
	Properties map[string]ToolProperty `json:"properties"`
}

// ToolProperty describes a single property in a tool's parameter schema.
type ToolProperty struct {
	Type        PropertyType `json:"type,omitempty"`
	Description string       `json:"description,omitempty"`
	Enum        []any        `json:"enum,omitempty"`
}

// PropertyType is a JSON schema type that can be a single string or an array of strings.
// Single-element marshals as a bare string: "string"
// Multi-element marshals as an array: ["string","null"]
type PropertyType []string

// MarshalJSON implements json.Marshaler.
// Single-element types marshal as a bare JSON string.
// Multi-element types marshal as a JSON array of strings.
func (pt PropertyType) MarshalJSON() ([]byte, error) {
	if len(pt) == 1 {
		return json.Marshal(pt[0])
	}
	return json.Marshal([]string(pt))
}

// UnmarshalJSON implements json.Unmarshaler.
// Accepts both a bare JSON string and an array of strings.
func (pt *PropertyType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*pt = PropertyType{s}
		return nil
	}
	var ss []string
	if err := json.Unmarshal(data, &ss); err != nil {
		return err
	}
	*pt = ss
	return nil
}

// ToolCall represents a tool call requested by the model.
type ToolCall struct {
	ID       string           `json:"id,omitempty"`
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction describes which function to call and with what arguments.
type ToolCallFunction struct {
	Index     int            `json:"index"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}
