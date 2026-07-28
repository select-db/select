package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"backend/db"
	"backend/internal/api/query"
	"backend/internal/authz"
	"backend/internal/syncer/types"
)

// Entity is the per-resource descriptor the generator emits: the URL plural, the
// query.Resource (table + exposed fields) the read handlers run over, the
// required workspace actions per op, and the syncer Apply/ApplyDelete the write
// handlers delegate to. Everything else is generic.
type Entity struct {
	Singular string
	Plural   string
	Table    string // short table name carried on the sync commit
	Resource query.Resource
	Requires map[string][]string

	Apply       func(ctx context.Context, userID string, c types.Commit, lastPulledAt time.Time) (bool, *types.RestoredItem, error)
	ApplyDelete func(ctx context.Context, userID string, c types.Commit) (bool, *types.RestoredItem, error)
}

// Per-op request-rate ceilings (requests/minute), mirroring the hand-written
// routes' pattern of cheap reads and stricter writes. wrap keys them by principal.
const (
	readRate  = 120
	writeRate = 30
)

// Register wires an entity's declared ops onto the mux as REST routes. wrap is
// the shared middleware chain the caller supplies — authenticated ∘
// WorkspaceFromHeader ∘ limited(perMinute) — parameterized by rate so this
// package reuses the app's auth/membership/rate-limit without importing it.
// ServeMux's {id} pattern carries the path param; gate() inside each handler
// enforces the op's `requires` actions.
func Register(mux *http.ServeMux, wrap func(perMinute int, h http.Handler) http.Handler, entities []Entity) {
	for _, e := range entities {
		if hasOp(e, "list") {
			mux.Handle("GET /"+e.Plural, wrap(readRate, listHandler(e)))
		}
		if hasOp(e, "get") {
			mux.Handle("GET /"+e.Plural+"/{id}", wrap(readRate, getHandler(e)))
		}
		if hasOp(e, "create") {
			mux.Handle("POST /"+e.Plural, wrap(writeRate, createHandler(e)))
		}
		if hasOp(e, "update") {
			mux.Handle("PATCH /"+e.Plural+"/{id}", wrap(writeRate, updateHandler(e)))
		}
		if hasOp(e, "delete") {
			mux.Handle("DELETE /"+e.Plural+"/{id}", wrap(writeRate, deleteHandler(e)))
		}
	}
}

func hasOp(e Entity, op string) bool { _, ok := e.Requires[op]; return ok }

// gate enforces the op's required workspace actions: the owner bypasses, anyone
// else must hold every listed action. Membership itself is already enforced by
// the middleware, so an op with no required actions is open to any member.
func gate(a authz.Actor, actions []string) bool {
	if a.IsOwner() {
		return true
	}
	for _, act := range actions {
		if !a.Can(act) {
			return false
		}
	}
	return true
}

func listHandler(e Entity) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a := authz.ActorOf(r)
		if !gate(a, e.Requires["list"]) {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		q := r.URL.Query()
		limit, _ := strconv.Atoi(q.Get("limit"))
		page, err := query.List(r.Context(), db.GetDB(), e.Resource, query.Request{
			WorkspaceID: a.WorkspaceID,
			Filter:      q.Get("$filter"),
			Sort:        q.Get("sort"),
			Limit:       limit,
			Cursor:      q.Get("cursor"),
		})
		if err != nil {
			writeQueryError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, page)
	}
}

func getHandler(e Entity) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a := authz.ActorOf(r)
		if !gate(a, e.Requires["get"]) {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		row, found, err := query.Get(r.Context(), db.GetDB(), e.Resource, a.WorkspaceID, r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if !found {
			writeError(w, http.StatusNotFound, e.Singular+" not found")
			return
		}
		writeJSON(w, http.StatusOK, row)
	}
}

func createHandler(e Entity) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a := authz.ActorOf(r)
		if !gate(a, e.Requires["create"]) {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		body, ok := decodeBody(w, r)
		if !ok {
			return
		}
		id, _ := body["id"].(string)
		if id == "" {
			writeError(w, http.StatusBadRequest, "id is required")
			return
		}
		if _, found, err := query.Get(r.Context(), db.GetDB(), e.Resource, a.WorkspaceID, id); err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		} else if found {
			writeError(w, http.StatusConflict, e.Singular+" already exists")
			return
		}
		applied, err := applyWrite(r, a, e, "create", id, body)
		if err != nil {
			writeWriteError(w, e, err)
			return
		}
		if !applied { // LWW rejected it (e.g. a soft-deleted row holds this id)
			writeError(w, http.StatusConflict, e.Singular+" already exists")
			return
		}
		respondEntity(w, r, a, e, id, http.StatusCreated)
	}
}

func updateHandler(e Entity) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a := authz.ActorOf(r)
		if !gate(a, e.Requires["update"]) {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		id := r.PathValue("id")
		if _, found, err := query.Get(r.Context(), db.GetDB(), e.Resource, a.WorkspaceID, id); err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		} else if !found {
			writeError(w, http.StatusNotFound, e.Singular+" not found")
			return
		}
		body, ok := decodeBody(w, r)
		if !ok {
			return
		}
		body["id"] = id // the path id is authoritative
		applied, err := applyWrite(r, a, e, "update", id, body)
		if err != nil {
			writeWriteError(w, e, err)
			return
		}
		if !applied {
			writeError(w, http.StatusConflict, "the "+e.Singular+" was changed concurrently; retry")
			return
		}
		respondEntity(w, r, a, e, id, http.StatusOK)
	}
}

func deleteHandler(e Entity) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a := authz.ActorOf(r)
		if !gate(a, e.Requires["delete"]) {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		id := r.PathValue("id")
		if _, found, err := query.Get(r.Context(), db.GetDB(), e.Resource, a.WorkspaceID, id); err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		} else if !found {
			writeError(w, http.StatusNotFound, e.Singular+" not found")
			return
		}
		if _, err := applyWrite(r, a, e, "delete", id, map[string]any{"id": id}); err != nil {
			writeWriteError(w, e, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// applyWrite delegates to the syncer's Apply/ApplyDelete via a synthesized
// commit stamped with server time (the server is authoritative, so LWW resolves
// in the request's favor barring a truly concurrent write). Reusing this path
// means the FK scope guard, tenancy check, soft-delete, and audit emission are
// exactly the sync path's.
func applyWrite(r *http.Request, a authz.Actor, e Entity, op, id string, body map[string]any) (bool, error) {
	c := types.Commit{
		ID:          id + ":" + op,
		CreatedAt:   time.Now(),
		Operation:   op,
		TableName:   e.Table,
		ObjectID:    id,
		Payload:     body,
		UserID:      a.UserID,
		WorkspaceID: a.WorkspaceID,
	}
	if op == "delete" {
		applied, _, err := e.ApplyDelete(r.Context(), a.UserID, c)
		return applied, err
	}
	applied, _, err := e.Apply(r.Context(), a.UserID, c, time.Time{})
	return applied, err
}

// respondEntity re-reads the row after a write and returns it, so the client
// sees the server-canonical shape (server-set updated_at, applied defaults).
func respondEntity(w http.ResponseWriter, r *http.Request, a authz.Actor, e Entity, id string, status int) {
	row, found, err := query.Get(r.Context(), db.GetDB(), e.Resource, a.WorkspaceID, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !found { // applied but not readable (raced with a delete) — report success without a body
		w.WriteHeader(status)
		return
	}
	writeJSON(w, status, row)
}

func decodeBody(w http.ResponseWriter, r *http.Request) (map[string]any, bool) {
	var m map[string]any
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return nil, false
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, true
}

// writeWriteError maps an Apply/ApplyDelete failure. These are predominantly
// client-caused (a cross-workspace FK, an invalid uuid, a stale row), but the
// path does not yet return typed errors, so we return a generic 422 rather than
// risk leaking an internal DB message. Precise write-validation messages are a
// follow-up (typing the apply errors like the $filter errors are typed).
func writeWriteError(w http.ResponseWriter, e Entity, _ error) {
	writeError(w, http.StatusUnprocessableEntity, "could not process the "+e.Singular)
}
