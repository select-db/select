package query

import (
	"encoding/base64"
	"strings"
)

// Cursor is an opaque keyset-pagination position: the sort-key value and id of
// the last row on a page. Encoded as base64url of "<value>|<id>". The id (a
// uuid, no '|') is split off from the right, so a sort value containing '|' is
// preserved. The value is typed against the current sort field when it is used,
// so the cursor itself stays a plain string.
type Cursor struct {
	Value string
	ID    string
}

func EncodeCursor(c Cursor) string {
	return base64.RawURLEncoding.EncodeToString([]byte(c.Value + "|" + c.ID))
}

// DecodeCursor parses a cursor produced by EncodeCursor. A malformed cursor is
// an InputError (the handler maps it to 400) rather than a server fault.
func DecodeCursor(s string) (Cursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return Cursor{}, inputErr("invalid cursor")
	}
	i := strings.LastIndexByte(string(raw), '|')
	if i < 0 {
		return Cursor{}, inputErr("invalid cursor")
	}
	return Cursor{Value: string(raw[:i]), ID: string(raw[i+1:])}, nil
}
