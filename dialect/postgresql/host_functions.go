package postgresql

import (
	"github.com/antlr4-go/antlr/v4"
	"github.com/selectDb/dialect/core"
)

// hostFunctions reach past the rows into the server itself: its filesystem, its
// large objects, another server. A statement calling one is not the plain read
// its shape suggests, so it takes manage as well as whatever it reads.
//
// PostgreSQL grants them separately, to a superuser or a role in
// pg_read_server_files, which is a second gate and not this one.
var hostFunctions = map[string]bool{
	"pg_read_file":         true,
	"pg_read_binary_file":  true,
	"pg_write_file":        true,
	"pg_file_write":        true,
	"pg_file_rename":       true,
	"pg_file_unlink":       true,
	"pg_ls_dir":            true,
	"pg_logdir_ls":         true,
	"pg_stat_file":         true,
	"lo_import":            true,
	"lo_export":            true,
	"dblink":               true,
	"dblink_connect":       true,
	"dblink_exec":          true,
	"dblink_connect_u":     true,
	"pg_reload_conf":       true,
	"pg_rotate_logfile":    true,
	"pg_terminate_backend": true,
}

// callsHostFunction reports whether the tokens between from and to call one of
// them.
func callsHostFunction(tokens *antlr.CommonTokenStream, from, to int) bool {
	return core.CallsAnyOf(tokens, from, to, hostFunctions, `"`)
}
