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
