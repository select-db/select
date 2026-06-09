package datasource

import (
	"context"
	"encoding/json"
	"time"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"

	"github.com/google/uuid"
	"github.com/selectDb/dialect/engine"
	"github.com/selectDb/toolkit/cache"
)

// ResolvedDatasource holds the decrypted, parsed datasource config ready for use.
type ResolvedDatasource struct {
	DBType string
	Name   string
	DSN    string
	SSH    *engine.ResolvedSSHConfig
	Pool   engine.PoolConfig
}

var dsCache = cache.New(cache.Options{
	MaxEntries: 20_000,
	TTL:        20 * time.Minute,
})

func cacheKey(workspaceID, id string) string {
	return workspaceID + ":" + id
}

// InvalidateCache drops a cached datasource entry. Call on upsert/delete.
func InvalidateCache(workspaceID, id string) {
	dsCache.Delete(cacheKey(workspaceID, id))
}

// GetOrLoadDatasource returns a cached ResolvedDatasource, fetching from DB on miss.
func GetOrLoadDatasource(ctx context.Context, id, workspaceID string) (*ResolvedDatasource, error) {
	key := cacheKey(workspaceID, id)
	if v, ok := dsCache.Get(key); ok {
		return v.(*ResolvedDatasource), nil
	}

	enc, err := getWrapper()
	if err != nil {
		return nil, err
	}
	parsedID, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}
	parsedWorkspaceID, err := uuid.Parse(workspaceID)
	if err != nil {
		return nil, err
	}

	row, err := db.Queries.GetDatasource(ctx, generated.GetDatasourceParams{
		ID:          db_types.NewJSONNullUUID(parsedID),
		WorkspaceID: db_types.NewJSONNullUUID(parsedWorkspaceID),
	})
	if err != nil {
		return nil, err
	}

	dsn, err := decryptField(ctx, enc, row.EncryptedDsn, fieldAAD(parsedWorkspaceID, parsedID, "dsn"))
	if err != nil {
		return nil, err
	}
	ssh, err := decryptField(ctx, enc, row.EncryptedSsh, fieldAAD(parsedWorkspaceID, parsedID, "ssh"))
	if err != nil {
		return nil, err
	}

	ds := &ResolvedDatasource{
		DBType: row.DbType.String,
		Name:   row.Name.String,
		DSN:    dsn,
		Pool: engine.PoolConfig{
			MaxOpenConns:    int(row.MaxOpenConns.Int64),
			MaxIdleConns:    int(row.MaxIdleConns.Int64),
			ConnMaxLifetime: time.Duration(row.ConnMaxLifetime.Int64) * time.Second,
			ConnMaxIdleTime: time.Duration(row.ConnMaxIdleTime.Int64) * time.Second,
		},
	}

	if ssh != "" {
		var sshCfg struct {
			Enabled    bool   `json:"enabled"`
			Host       string `json:"host"`
			Port       int    `json:"port"`
			User       string `json:"user"`
			AuthMethod string `json:"auth_method"`
			Password   string `json:"password"`
			PrivateKey string `json:"private_key"`
			HostKey    string `json:"host_key"`
		}
		if err := json.Unmarshal([]byte(ssh), &sshCfg); err == nil && sshCfg.Enabled {
			ds.SSH = &engine.ResolvedSSHConfig{
				Host:       sshCfg.Host,
				Port:       sshCfg.Port,
				User:       sshCfg.User,
				AuthMethod: sshCfg.AuthMethod,
				Password:   sshCfg.Password,
				PrivateKey: sshCfg.PrivateKey,
				HostKey:    sshCfg.HostKey,
			}
		}
	}

	dsCache.Set(key, ds)
	return ds, nil
}
