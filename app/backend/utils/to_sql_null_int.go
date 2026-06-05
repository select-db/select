package utils

import "database/sql"

func ToNullInt64(n *int32) sql.NullInt64 {
	if n == nil {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: int64(*n), Valid: true}
}
