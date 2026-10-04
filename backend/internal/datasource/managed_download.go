package datasource

import (
	"fmt"
	"io"
	"net/http"

	"backend/internal/authz"
	"backend/internal/datasource/managed/cellarclient"

	"github.com/selectDb/dialect/sqlite"
	"github.com/selectDb/toolkit"
)

// DownloadHandler sends a managed database's file. It hands over all the
// data, so it needs manage.
func DownloadHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor := authz.ActorOf(r)
		id := r.PathValue("id")
		if !actor.ManagesDatasource(id) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		resolved, err := GetOrLoadDatasource(r.Context(), id, actor.WorkspaceID)
		if err == nil && !sqlite.IsCellarDSN(resolved.DSN) {
			err = ErrNotFound
		}
		if err != nil {
			OpenError(w, err, "managed download", actor.WorkspaceID, id)
			return
		}
		databaseFile, err := cellarclient.Download(r.Context(), resolved.DSN)
		if err != nil {
			OpenError(w, err, "managed download", actor.WorkspaceID, id)
			return
		}
		defer func() { _ = databaseFile.Close() }()
		w.Header().Set("Content-Type", "application/vnd.sqlite3")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", toolkit.SafeFileName(resolved.Name, "database")+".db"))
		_, _ = io.Copy(w, databaseFile)
	}
}
