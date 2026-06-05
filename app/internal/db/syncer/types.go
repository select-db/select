package syncer

import "time"

// SyncRequest is sent to POST /sync/v1/sync.
type SyncRequest struct {
	PendingCommits []SyncCommit `json:"pending_commits"`
	LastPulledAt   *time.Time   `json:"last_pulled_at,omitempty"`
}

// SyncCommit matches backend expectation.
type SyncCommit struct {
	ID          string      `json:"id"`
	CreatedAt   time.Time   `json:"created_at"`
	Operation   string      `json:"operation"`
	TableName   string      `json:"table_name"`
	ObjectID    string      `json:"object_id"`
	Payload     interface{} `json:"payload"`
	UserID      string      `json:"user_id"`
	WorkspaceID string      `json:"workspace_id"`
}

// SyncResponse is the response from POST /sync/v1/sync.
type SyncResponse struct {
	Confirmed  []string           `json:"confirmed"`
	Restored   []SyncRestoredItem `json:"restored"`
	Changes    SyncChanges        `json:"changes"`
	ServerTime time.Time          `json:"server_time"`
}

// SyncRestoredItem is one server row returned when a commit was rejected (last-write-wins).
type SyncRestoredItem struct {
	ObjectID      string      `json:"object_id"`
	TableName     string      `json:"table_name"`
	ServerPayload interface{} `json:"server_payload"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

// SyncChanges contains entities in FK order: users → workspaces → workspace_to_user → roles → user_to_role → permissions.
// Each row is a raw map so it flows directly into applyRow without conversion.
type SyncChanges struct {
	Users           []map[string]any `json:"users"`
	Workspaces      []map[string]any `json:"workspaces"`
	WorkspaceToUser []map[string]any `json:"workspace_to_user"`
	Role            []map[string]any `json:"role"`
	UserToRole      []map[string]any `json:"user_to_role"`
	Permission      []map[string]any `json:"permission"`
}

