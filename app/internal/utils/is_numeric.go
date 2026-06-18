package utils

import "strings"

// IsNumeric checks if a string represents a number (int or float)
func IsNumeric(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}

	// Allow optional leading sign
	if s[0] == '+' || s[0] == '-' {
		s = s[1:]
	}

	if s == "" {
		return false
	}

	// Reject leading dot (like ".5")
	if s[0] == '.' {
		return false
	}

	dotCount := 0
	for _, c := range s {
		if c == '.' {
			dotCount++
			if dotCount > 1 {
				return false // More than one decimal point
			}
		} else if c < '0' || c > '9' {
			return false
		}
	}

	return true
}
