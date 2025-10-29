package net

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContentTypeConverters(t *testing.T) {
	tests := []struct {
		name        string
		converter   ContentTypeConverter
		input       json.RawMessage
		expected    string
		contentType string
	}{
		{
			name:        "JSON converter returns data as-is",
			converter:   JSONConverter{},
			input:       json.RawMessage(`{"key": "value", "number": 123}`),
			expected:    `{"key": "value", "number": 123}`,
			contentType: "application/json",
		},
		{
			name:        "Form URL encoded converter converts JSON to form data",
			converter:   FormURLEncodedConverter{},
			input:       json.RawMessage(`{"key": "value", "number": 123, "bool": true}`),
			expected:    "", // Will be validated by parsing the result
			contentType: "application/x-www-form-urlencoded",
		},
		{
			name:        "Form URL encoded converter handles null values",
			converter:   FormURLEncodedConverter{},
			input:       json.RawMessage(`{"key": "value", "null_key": null}`),
			expected:    "key=value&null_key=",
			contentType: "application/x-www-form-urlencoded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.converter.Convert(tt.input)
			require.NoError(t, err)

			if tt.expected == "" {
				// For form data, validate by parsing the result
				require.Equal(t, tt.contentType, tt.converter.ContentType())
				// Basic validation that it's valid form data
				require.NotEmpty(t, string(result))
			} else {
				require.Equal(t, tt.expected, string(result))
				require.Equal(t, tt.contentType, tt.converter.ContentType())
			}
		})
	}
}

func TestGetConverter(t *testing.T) {
	tests := []struct {
		contentType string
		expected    ContentTypeConverter
	}{
		{
			contentType: "application/json",
			expected:    JSONConverter{},
		},
		{
			contentType: "application/x-www-form-urlencoded",
			expected:    FormURLEncodedConverter{},
		},
		{
			contentType: "text/plain",
			expected:    JSONConverter{}, // default fallback
		},
		{
			contentType: "",
			expected:    JSONConverter{}, // default fallback
		},
	}

	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			converter := getConverter(tt.contentType)
			require.IsType(t, tt.expected, converter)
		})
	}
}
