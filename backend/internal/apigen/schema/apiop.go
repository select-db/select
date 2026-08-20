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

// RespBody classifies the body a response carries, so the OpenAPI emitter can
// attach the right schema without the schema package depending on OpenAPI types.
type RespBody int

const (
	RespNone       RespBody = iota // no body (204)
	RespResource                   // the entity object (200/201)
	RespPage                       // the {data, next_cursor} page (list 200)
	RespError                      // the {error} envelope (4xx/5xx)
	RespValidation                 // the {error, fields} envelope (422)
)

// APIResponse is one response an op can return: its status, a human description
// (using the word "resource", which the OpenAPI emitter swaps for the model
// name), and the body it carries.
type APIResponse struct {
	Status int
	Desc   string
	Body   RespBody
}

// ResponseCodes is the SINGLE SOURCE of every response an @app.api op can return.
// The OpenAPI emitter documents exactly these, and an alignment test asserts the
// generated handlers return exactly these (plus the 401 the auth middleware
// adds) — so the docs and the API can't drift: change a handler's status set and
// this must change too, or the build fails.
func ResponseCodes(op string) []APIResponse {
	shared := []APIResponse{
		{401, "Missing or invalid authentication.", RespError},
		{403, "Authenticated but lacks the required workspace action.", RespError},
		{500, "Unexpected server error.", RespError},
	}
	add := func(rs ...APIResponse) []APIResponse { return append(shared, rs...) }
	switch op {
	case "list":
		return add(
			APIResponse{200, "A page of resources.", RespPage},
			APIResponse{400, "Invalid $filter, sort, or cursor.", RespError},
		)
	case "get":
		return add(
			APIResponse{200, "The resource.", RespResource},
			APIResponse{404, "No such resource in this workspace.", RespError},
		)
	case "create":
		return add(
			APIResponse{201, "The created resource.", RespResource},
			APIResponse{409, "A resource with this id already exists.", RespError},
			APIResponse{422, "The body failed validation or references a row outside this workspace.", RespValidation},
		)
	case "update":
		return add(
			APIResponse{200, "The updated resource.", RespResource},
			APIResponse{404, "No such resource in this workspace.", RespError},
			APIResponse{409, "The resource was modified concurrently; retry.", RespError},
			APIResponse{422, "The body failed validation or references a row outside this workspace.", RespValidation},
		)
	case "delete":
		return add(
			APIResponse{204, "Deleted.", RespNone},
			APIResponse{404, "No such resource in this workspace.", RespError},
			APIResponse{422, "The resource could not be deleted.", RespError},
		)
	}
	return shared
}
