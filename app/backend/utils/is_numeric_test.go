package utils

import "testing"

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"123", true},
		{"123.45", true},
		{"-123", true},
		{"+123.45", true},
		{"0.5", true},
		{".5", false}, // Leading dot not allowed
		{"123.", true},
		{"abc", false},
		{"12a34", false},
		{"12.34.56", false}, // Multiple dots
		{"", false},
		{"  123  ", true}, // Spaces trimmed
		{"-", false},
		{"+", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := IsNumeric(tt.input)
			if result != tt.expected {
				t.Errorf("IsNumeric(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}
