package main

import "testing"

func TestEncodeBencode(t *testing.T) {
	tests := []struct {
		expected string
		name     string
		input    any
	}{
		{
			expected: "5:hello",
			name:     "string",
			input:    "hello",
		},
		{
			expected: "i5e",
			name:     "positve int",
			input:    5,
		},
		{
			expected: "i-5e",
			name:     "negative int",
			input:    -5,
		},
		{
			expected: "l5:helloi52ee",
			name:     "list",
			input:    []any{"hello", 52},
		},
		{
			expected: "d3:foo3:bar5:helloi52ee",
			name:     "dictionary",
			input: map[string]any{
				"foo":   "bar",
				"hello": 52,
			},
		},
	}

	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			actualBytes, err := encodeBencode(ts.input)
			if err != nil {
				t.Fatalf("unexpected error: %s", err.Error())
			}
			actual := string(actualBytes)
			if actual != ts.expected {
				t.Fatalf("unexpected value: %v instead of %v", actual, ts.expected)
			}
		})
	}
}
