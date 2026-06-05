package toolkit

import (
	"os"
)

func Debug(identifier string, f func()) {
	if os.Getenv("DEBUG") == identifier || os.Getenv("DEBUG") == "*" {
		f()
	}
}
