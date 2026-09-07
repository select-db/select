package graph

import (
	"strings"
	"testing"
)

func TestDatabaseFolderName(t *testing.T) {
	tests := []struct {
		why  string
		name string
		want string
	}{
		{
			why:  "a plain name is left exactly as it was typed",
			name: "analytics",
			want: "analytics",
		},
		{
			why:  "case is part of the name, not something to normalize away",
			name: "Analytics",
			want: "Analytics",
		},
		{
			why:  "so are spaces: the folder is what the user will be shown",
			name: "Prod Analytics",
			want: "Prod Analytics",
		},
		{
			why:  "and so are accents and non-latin scripts",
			name: "café-données",
			want: "café-données",
		},
		{
			why:  "a separator would make two path components out of one name",
			name: "sales/eu",
			want: "sales-eu",
		},
		{
			why:  "the same on Windows, which takes the other slash too",
			name: `sales\eu`,
			want: "sales-eu",
		},
		{
			why:  "the rest of what Windows refuses in a name",
			name: `a:b*c?d"e<f>g|h`,
			want: "a-b-c-d-e-f-g-h",
		},
		{
			why:  "control characters are dropped rather than replaced",
			name: "ana\x00lyt\x1fics",
			want: "analytics",
		},
		{
			why:  "a leading dot would hide the database from its own workspace",
			name: ".analytics",
			want: "analytics",
		},
		{
			why:  "Windows drops a trailing dot on the way in, so we drop it first",
			name: "analytics.",
			want: "analytics",
		},
		{
			why:  "and a trailing space, for the same reason",
			name: "analytics ",
			want: "analytics",
		},
		{
			why:  "surrounding whitespace is not part of what anyone meant to type",
			name: "  analytics  ",
			want: "analytics",
		},
		{
			why:  "an interior dot is ordinary",
			name: "analytics.v2",
			want: "analytics.v2",
		},
		{
			why:  "a name with nothing usable in it still has to land somewhere",
			name: "///",
			want: "---",
		},
		{
			why:  "empty means empty",
			name: "",
			want: fallbackDatabaseFolderName,
		},
		{
			why:  "whitespace is empty too",
			name: "   ",
			want: fallbackDatabaseFolderName,
		},
		{
			why:  "so is a name that is only the parts we take out",
			name: "...",
			want: fallbackDatabaseFolderName,
		},
		{
			why:  "a relative path is a name we cannot use",
			name: "..",
			want: fallbackDatabaseFolderName,
		},
		{
			why:  "Windows keeps some names for devices, whatever the case",
			name: "CON",
			want: "CON-db",
		},
		{
			why:  "including when something follows a dot",
			name: "nul.sql",
			want: "nul.sql-db",
		},
		{
			why:  "and the numbered ones",
			name: "com4",
			want: "com4-db",
		},
		{
			why:  "a name that merely starts with a reserved one is fine",
			name: "console",
			want: "console",
		},
	}

	for _, tc := range tests {
		t.Run(tc.why, func(t *testing.T) {
			if got := DatabaseFolderName(tc.name); got != tc.want {
				t.Errorf("DatabaseFolderName(%q) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

func TestDatabaseFolderNameFitsTheFilesystem(t *testing.T) {
	// Every filesystem we target stops at 255 bytes for one path component.
	long := strings.Repeat("a", 300)
	got := DatabaseFolderName(long)

	if len(got) != maxFolderNameBytes {
		t.Errorf("length = %d, want %d", len(got), maxFolderNameBytes)
	}
}

func TestDatabaseFolderNameCutsOnARuneBoundary(t *testing.T) {
	// Truncating mid-rune would leave a name that is not valid UTF-8, which the
	// graph would carry all the way to the UI.
	long := strings.Repeat("é", 200) // two bytes each, so the limit falls mid-rune
	got := DatabaseFolderName(long)

	if len(got) > maxFolderNameBytes {
		t.Fatalf("length = %d, want at most %d", len(got), maxFolderNameBytes)
	}
	if !isValidUTF8(got) {
		t.Errorf("DatabaseFolderName(%d× é) is not valid UTF-8: %q", 200, got)
	}
}

func isValidUTF8(s string) bool {
	for _, r := range s {
		if r == '�' {
			return false
		}
	}
	return true
}
