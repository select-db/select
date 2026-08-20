package httpapi

import (
	"sort"
	"strings"
	"testing"

	"backend/internal/apigen/schema"
)

// handlerStatuses returns the HTTP status codes a rendered handler can emit,
// resolving the rest.* response helpers to the codes they produce.
func handlerStatuses(src string) map[int]bool {
	direct := map[string]int{
		"http.StatusOK": 200, "http.StatusCreated": 201, "http.StatusNoContent": 204,
		"http.StatusBadRequest": 400, "http.StatusForbidden": 403, "http.StatusNotFound": 404,
		"http.StatusConflict": 409, "http.StatusUnprocessableEntity": 422,
		"http.StatusInternalServerError": 500,
	}
	m := map[int]bool{}
	for name, code := range direct {
		if strings.Contains(src, name) {
			m[code] = true
		}
	}
	if strings.Contains(src, "WriteQueryError") { // 400 on bad $filter/sort/cursor, else 500
		m[400], m[500] = true, true
	}
	if strings.Contains(src, "WriteValidationError") { // body/FK validation
		m[422] = true
	}
	if strings.Contains(src, "WriteWriteError") { // apply/FK failure
		m[422] = true
	}
	return m
}

// TestDocAPIStatusAlignment is the guard that keeps the OpenAPI doc and the
// generated handlers pixel-aligned. For every op, the status codes the handler
// emits (plus the 401 the auth middleware adds, which isn't visible in the
// handler file) must be exactly the set schema.ResponseCodes documents. If a
// handler template starts returning a new code, or the doc lists a code no
// handler returns, this fails — so doc and API can't drift.
func TestDocAPIStatusAlignment(t *testing.T) {
	files := filesByName(t, roleEntity()) // role declares all five ops
	for _, op := range []string{"list", "get", "create", "update", "delete"} {
		got := handlerStatuses(mustFile(t, files, "role/"+op+".go"))
		got[401] = true // auth middleware

		want := map[int]bool{}
		for _, r := range schema.ResponseCodes(op) {
			want[r.Status] = true
		}

		if !sameStatusSet(got, want) {
			t.Fatalf("%s: handler emits %v but doc documents %v", op, sortedKeys(got), sortedKeys(want))
		}
	}
}

func sameStatusSet(a, b map[int]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func sortedKeys(m map[int]bool) []int {
	ks := make([]int, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Ints(ks)
	return ks
}
