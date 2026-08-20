package codegen

import "strings"

// Pascal turns snake_case into PascalCase (role -> Role, user_to_role -> UserToRole).
func Pascal(s string) string {
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}

// Plural is the collection path segment for a resource name (role -> roles).
// Simple English suffixing is enough for the current entity names; a name that
// already ends in "s" is left as-is.
func Plural(name string) string {
	if strings.HasSuffix(name, "s") {
		return name
	}
	return name + "s"
}

// GoField mirrors sqlc's column -> Go field naming for this repo (only "id" is an
// initialism: WorkspaceID, RoleID, ClientIp). `go build` against db/generated is
// the check that this stays correct.
func GoField(col string) string {
	parts := strings.Split(col, "_")
	for i, p := range parts {
		switch {
		case p == "id":
			parts[i] = "ID"
		case p != "":
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}
