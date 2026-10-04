package cellar

import "regexp"

// idPattern is what a cellar id must match: the database refuses any other
// cellar_id on a managed datasource (datasource_managed_check).
var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// ValidID reports whether id can name a cellar.
func ValidID(id string) bool { return idPattern.MatchString(id) }
