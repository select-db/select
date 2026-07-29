package schema

import "net/http"

// REST semantics of an @app.api op, single-sourced here so the handler emitter
// (httpapi) and the OpenAPI emitter agree on method/path/verb and the write set
// without each hand-maintaining its own op table. The op is the free string
// parsed from the @app.api spec ("list", "get", "create", "update", "delete").

// RESTMethod is the HTTP method an op maps to, in canonical uppercase. The
// OpenAPI emitter lowercases it for the spec.
func RESTMethod(op string) string {
	switch op {
	case "create":
		return http.MethodPost
	case "update":
		return http.MethodPatch
	case "delete":
		return http.MethodDelete
	default: // list, get
		return http.MethodGet
	}
}

// OnCollection reports whether an op acts on the collection path (list, create)
// rather than the item /{id} path (get, update, delete).
func OnCollection(op string) bool {
	return op == "list" || op == "create"
}

// IsWriteOp reports whether an op mutates a row (create, update, delete). The
// read ops (list, get) carry no write side effects and no required actions.
func IsWriteOp(op string) bool {
	switch op {
	case "create", "update", "delete":
		return true
	}
	return false
}

// HasOp reports whether the entity declares the given API op.
func HasOp(e Entity, op string) bool {
	for _, o := range e.API {
		if o.Op == op {
			return true
		}
	}
	return false
}
