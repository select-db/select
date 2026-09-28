package toolkit

import "strings"

// SafeFileName keeps name to characters every file system accepts, replacing
// the others with '_'. A name left empty once trimmed becomes fallback.
func SafeFileName(name, fallback string) string {
	cleaned := strings.Map(func(char rune) rune {
		if char < 0x20 || strings.ContainsRune(`/\:*?"<>|`, char) {
			return '_'
		}
		return char
	}, strings.TrimSpace(name))
	if cleaned == "" {
		return fallback
	}
	return cleaned
}
