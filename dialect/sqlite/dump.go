package sqlite

// DumpSchema returns ("", false) for SQLite: the schema is already reconstructed
// from sqlite_master which is the authoritative source, so no native tool is needed.
func (d *Dialect) DumpSchema(_ string) (string, bool) { return "", false }

// DumpSchemaWarning returns an empty string: the SQLite fallback (sqlite_master) is
// already authoritative, so no warning is needed.
func (d *Dialect) DumpSchemaWarning() string { return "" }
