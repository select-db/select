package cellar

import (
	"encoding/base64"
	"encoding/json"
)

// Grant is what one cellar request may do. The cellar knows no users and reads
// no Postgres, so the backend sends all of it in GrantHeader.
type Grant struct {
	DatasourceID string `json:"-"` // the path's id
	WorkspaceID  string `json:"workspace_id"`
	CellarID     string `json:"cellar_id"`
	MaxBytes     int64  `json:"max_bytes"`
	MaxInFlight  int    `json:"max_in_flight"` // statements the workspace may run at once
}

// GrantHeader carries a Grant as base64url JSON.
const GrantHeader = "X-Cellar-Grant"

// Audience is the audience of the backend's tokens for a cellar, so a user
// token never opens one.
const Audience = "selectdb-cellar"

// Encode is the grant as the GrantHeader value.
func (grant Grant) Encode() (string, error) {
	b, err := json.Marshal(grant)
	return base64.RawURLEncoding.EncodeToString(b), err
}

func decodeGrant(v string) (Grant, error) {
	var grant Grant
	b, err := base64.RawURLEncoding.DecodeString(v)
	if err == nil {
		err = json.Unmarshal(b, &grant)
	}
	return grant, err
}
