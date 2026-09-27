package utils

import "log"

// LogWithRef logs detail under a fresh ref and returns the ref: the only part
// of an internal error a caller may see.
func LogWithRef(detail string) string {
	ref := GenerateRequestID()
	log.Printf("ref=%s: %s", ref, detail)
	return ref
}
