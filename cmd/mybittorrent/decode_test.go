package main

import (
	"encoding/json"
	"testing"
)

func TestDecodeBencode(t *testing.T) {
	tests := []struct {
		expected      string
		name          string
		input         string
		expectedIndex int
	}{
		{
			name:          "string",
			input:         "5:hello",
			expected:      "\"hello\"",
			expectedIndex: 7,
		},
		{
			name:          "positive_int",
			input:         "i52e",
			expected:      "52",
			expectedIndex: 4,
		},
		{
			name:          "positive_int",
			input:         "i-52e",
			expected:      "-52",
			expectedIndex: 5,
		},
		{
			name:          "list",
			input:         "l5:helloi52ee",
			expected:      "[\"hello\",52]",
			expectedIndex: 13,
		},
		{
			name:          "dictionary",
			expected:      `{"foo":"bar","hello":52}`,
			input:         "d3:foo3:bar5:helloi52ee",
			expectedIndex: 23,
		},
	}

	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			actual, actualIndex, err := decodeBencode([]byte(ts.input))
			if err != nil {
				t.Fatalf("unexpected error: %s", err.Error())
			}
			actualJsonBytes, err := json.Marshal(actual)
			if err != nil {
				t.Fatalf("unexpected error when marshaling result to Json: %s", err.Error())
			}
			actualJson := string(actualJsonBytes)
			if actualJson != ts.expected {
				t.Fatalf("unexpected value: %v instead of %v", actualJson, ts.expected)
			}
			if actualIndex != ts.expectedIndex {
				t.Fatalf("unexpected index: %d instead of %d", actualIndex, ts.expectedIndex)
			}
		})
	}
}
