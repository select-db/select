package datasource

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"backend/internal/authz"
	"backend/internal/datasource/cellarclient"

	"github.com/selectDb/dialect/sqlite"
)

// DownloadHandler sends a managed database's file. It hands over all the
// data, so it needs manage.
func DownloadHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor := authz.ActorOf(r)
		id := r.PathValue("id")
		if !actor.IsOwner() && !actor.CanManage(id) {
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
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", safeFileName(resolved.Name)+".db"))
		_, _ = io.Copy(w, databaseFile)
	}
}

// safeFileName keeps a download's name to characters every file system accepts.
func safeFileName(name string) string {
	cleaned := strings.Map(func(char rune) rune {
		if char < 0x20 || strings.ContainsRune(`/\:*?"<>|`, char) {
			return '_'
		}
		return char
	}, strings.TrimSpace(name))
	if cleaned == "" {
		return "database"
	}
	return cleaned
}
