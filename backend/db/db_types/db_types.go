package db_types

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
)

/**
*	String
 */
type JSONNullString struct {
	sql.NullString
}

func (ns JSONNullString) MarshalJSON() ([]byte, error) {
	if ns.Valid {
		return json.Marshal(ns.String)
	}
	return json.Marshal(nil)
}
func NewJSONNullString(s string) JSONNullString {
	return JSONNullString{
		NullString: sql.NullString{
			String: s,
			Valid:  true,
		},
	}
}
func (ns JSONNullString) ValueOrEmpty() string {
	if !ns.Valid {
		return ""
	}
	return ns.String
}
func (ns JSONNullString) Ptr() *string {
	if !ns.Valid {
		return nil
	}
	return &ns.String
}

/**
*	UUID
 */
type JSONNullUUID struct {
	uuid.NullUUID
}

func (nu JSONNullUUID) MarshalJSON() ([]byte, error) {
	if nu.Valid {
		return json.Marshal(nu.UUID)
	}
	return json.Marshal(nil)
}
func NewJSONNullUUID(id uuid.UUID) JSONNullUUID {
	return JSONNullUUID{
		NullUUID: uuid.NullUUID{
			UUID:  id,
			Valid: true,
		},
	}
}
func (nu JSONNullUUID) String() string {
	if !nu.Valid {
		return ""
	}
	return nu.UUID.String()
}
func NewJSONNullUUIDFromString(s string) (JSONNullUUID, error) {
	u, err := uuid.Parse(s)
	if err != nil {
		return JSONNullUUID{}, err
	}
	return JSONNullUUID{
		NullUUID: uuid.NullUUID{
			UUID:  u,
			Valid: true,
		},
	}, nil
}

/**
*	Time
 */
type JSONNullTime struct {
	sql.NullTime
}

func (nt JSONNullTime) MarshalJSON() ([]byte, error) {
	if nt.Valid {
		return json.Marshal(nt.Time)
	}
	return json.Marshal(nil)
}
func (nt JSONNullTime) ValueOrZero() time.Time {
	if !nt.Valid {
		return time.Time{}
	}
	return nt.Time
}
func NewJSONNullTimeFromTime(t time.Time) JSONNullTime {
	return JSONNullTime{
		NullTime: sql.NullTime{
			Time:  t,
			Valid: true,
		},
	}
}

/**
*	Int64
 */
type JSONNullInt64 struct {
	sql.NullInt64
}

func (ni JSONNullInt64) MarshalJSON() ([]byte, error) {
	if ni.Valid {
		return json.Marshal(ni.Int64)
	}
	return json.Marshal(nil)
}
func NewJSONNullInt64(i int64) JSONNullInt64 {
	return JSONNullInt64{
		NullInt64: sql.NullInt64{
			Int64: i,
			Valid: true,
		},
	}
}

/**
*	Bool
 */
type JSONNullBool struct {
	sql.NullBool
}

/*
*

	Inet
*/
type JSONNullInet struct {
	pqtype.Inet
}

func (ni JSONNullInet) MarshalJSON() ([]byte, error) {
	v, err := ni.Value()
	if err != nil {
		return json.Marshal(nil)
	}
	if v == nil {
		return json.Marshal(nil)
	}
	return json.Marshal(v)
}
func NewJSONNullInet(ip pqtype.Inet) JSONNullInet {
	if !ip.Valid {
		return JSONNullInet{
			Inet: pqtype.Inet{
				Valid: false,
			},
		}
	}

	return JSONNullInet{
		Inet: pqtype.Inet{
			IPNet: ip.IPNet,
			Valid: true,
		},
	}
}
