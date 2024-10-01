package main

import (
	"testing"
)

func TestDecodeBencode(t *testing.T) {
	tests := []struct {
		expected any
		name     string
		input    string
	}{
		{
			name:     "string",
			input:    "5:hello",
			expected: "hello",
		},
		{
			name:     "positive_int",
			input:    "i52e",
			expected: 52,
		},
		{
			name:     "positive_int",
			input:    "i-52e",
			expected: -52,
		},
	}

	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			actual, err := decodeBencode(ts.input)
			if err != nil {
				t.Fatalf("unexpected error: %s", err.Error())
			}
			if actual != ts.expected {
				t.Fatalf("unexpected value: %v+ instead of %v+", actual, ts.expected)
			}
		})
	}
}
