package graph

import (
	"strings"
	"unicode"
)

// A database is a directory, and the directory's name is the database's name.
// There is nowhere else it is written down, so the translation from what
// someone types to what lands on disk changes as little as it can get away
// with: case, spaces and accents survive, because the name they typed is the
// name they will be shown. Only what a filesystem refuses is taken out.

// maxFolderNameBytes is the per-component limit on every filesystem we target.
const maxFolderNameBytes = 255

// fallbackDatabaseFolderName is used when a name has nothing left in it once
// the illegal parts are gone.
const fallbackDatabaseFolderName = "database"

// windowsReservedNames cannot be used for a directory on Windows, whatever the
// case and whatever follows a dot. A workspace is meant to be checked out
// anywhere, so a name refused on one platform is refused on all of them.
var windowsReservedNames = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true,
	"com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true,
	"lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// DatabaseFolderName is the directory name a database called name is stored
// under. The result is always usable as a single path component on Linux, macOS
// and Windows; it is never empty, and never one of the names those platforms
// keep for themselves.
func DatabaseFolderName(name string) string {
	cleaned := strings.Map(func(r rune) rune {
		switch r {
		// Reserved by a path or by Windows. Replaced rather than dropped, so
		// "sales/eu" reads as two words rather than running them together.
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return '-'
		}
		// Control characters, which no filesystem wants and no one can see.
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, name)

	// A leading dot hides the directory, and the workspace walk skips some of
	// what it would then look like. Trailing dots and spaces are dropped by
	// Windows on the way in, which would leave disk and graph disagreeing.
	cleaned = strings.Trim(cleaned, " \t.")

	cleaned = truncateBytes(cleaned, maxFolderNameBytes)

	// Truncation can uncover a new trailing dot or space.
	cleaned = strings.Trim(cleaned, " \t.")

	if cleaned == "" {
		return fallbackDatabaseFolderName
	}

	// "con.sql" is refused by Windows as surely as "con".
	stem, _, _ := strings.Cut(cleaned, ".")
	if windowsReservedNames[strings.ToLower(stem)] {
		return cleaned + "-db"
	}

	return cleaned
}

// truncateBytes cuts s to at most limit bytes, on a rune boundary so the result
// is still valid UTF-8.
func truncateBytes(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	cut := limit
	for cut > 0 && !isRuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}

// isRuneStart reports whether b begins a UTF-8 rune rather than continuing one.
func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }
