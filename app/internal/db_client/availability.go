package db_client

import "selectDb/internal/desktop"

// Tells the frontend what an operation just learned about a database, so the
// indicator is only ever as stale as the last thing that touched it. An empty
// errMsg means reachable.
//
// Two rules decide what may be reported. Reachability is claimed only where a
// round trip happened: a pooled handle proves nothing on its own, and a
// proxified database opens no local connection at all. And a failing statement
// is not a failing database, so only connection-level outcomes belong here --
// a syntax error means the database answered.
func emitAvailability(id, errMsg string) {
	entry := map[string]interface{}{"id": id}
	if errMsg != "" {
		entry["error"] = errMsg
	}

	desktop.Emit("databaseAvailability", map[string]interface{}{
		"databases": []map[string]interface{}{entry},
	})
}
