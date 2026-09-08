package db_client

import "selectDb/internal/desktop"

// What the database indicator in the UI is told, and by whom.
//
// The indicator is only as live as the operations that report to it. Every
// operation that learns whether a database can be reached says so through
// EmitAvailability rather than emitting its own event, so a new way of reaching
// a database updates the dot by calling one function rather than by remembering
// to copy an event payload.
//
// Two rules decide what may be reported:
//
// Reachability is claimed only where a round trip actually happened. Opening a
// pooled connection proves nothing on its own -- the pool hands back a handle
// it has not used -- and a proxified database opens no local connection at all.
// A successful ping, a metadata fetch, a stream that started: those are round
// trips.
//
// A failing statement is not a failing database. Only connection-level outcomes
// belong here; a syntax error or a permission denial leaves the dot alone,
// because the database answered.

// Availability is one database's reachability as an operation just found it. An
// empty Error means reachable.
type Availability struct {
	ID    string
	Error string
}

// EmitAvailability tells the frontend what one or more operations just learned.
func EmitAvailability(reports ...Availability) {
	if len(reports) == 0 {
		return
	}

	databases := make([]map[string]interface{}, 0, len(reports))
	for _, report := range reports {
		entry := map[string]interface{}{"id": report.ID}
		if report.Error != "" {
			entry["error"] = report.Error
		}
		databases = append(databases, entry)
	}

	desktop.Emit("databaseAvailability", map[string]interface{}{
		"databases": databases,
	})
}
