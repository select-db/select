package types

import (
	"encoding/json"
	"time"
)

// ToRestoredPayload converts a typed row to the interface{} form used in RestoredItem.ServerPayload
// (marshal then unmarshal so the API returns a JSON object, not a custom type).
func ToRestoredPayload(row interface{}) (interface{}, error) {
	payloadBytes, err := json.Marshal(row)
	if err != nil {
		return nil, err
	}
	var out interface{}
	if err := json.Unmarshal(payloadBytes, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Commit matches the app's mutation commit shape.
type Commit struct {
	ID          string      `json:"id"`
	CreatedAt   time.Time   `json:"created_at"`
	Operation   string      `json:"operation"`
	TableName   string      `json:"table_name"`
	ObjectID    string      `json:"object_id"`
	Payload     interface{} `json:"payload"`
	UserID      string      `json:"user_id"`
	WorkspaceID string      `json:"workspace_id"`
}

// SyncRequest is the body for POST /sync/v1/sync.
type SyncRequest struct {
	PendingCommits []Commit   `json:"pending_commits"`
	LastPulledAt   *time.Time `json:"last_pulled_at"`
}

// RestoredItem is one server row returned when a commit was rejected (last-write-wins).
type RestoredItem struct {
	ObjectID      string      `json:"object_id"`
	TableName     string      `json:"table_name"`
	ServerPayload interface{} `json:"server_payload"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

// SyncResponse is the response for POST /sync/v1/sync.
type SyncResponse struct {
	Confirmed  []string       `json:"confirmed"`
	Restored   []RestoredItem `json:"restored"`
	Changes    SyncChanges    `json:"changes"`
	ServerTime time.Time      `json:"server_time"`
}

// SyncChanges contains rows updated since last_pulled_at.
type SyncChanges struct {
	Users           []UserRow            `json:"users"`
	Workspaces      []WorkspaceRow       `json:"workspaces"`
	WorkspaceToUser []WorkspaceToUserRow `json:"workspace_to_user"`
	Role            []RoleRow            `json:"role"`
	UserToRole      []UserToRoleRow      `json:"user_to_role"`
	Permission      []PermissionRow      `json:"permission"`
}

// UserRow is a minimal user for sync.
type UserRow struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// WorkspaceRow is a workspace row for sync.
type WorkspaceRow struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	GitRemoteURL *string    `json:"git_remote_url"`
	OwnerID      *string    `json:"owner_id,omitempty"`
	UpdatedAt    time.Time  `json:"updated_at"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty"`
}

// WorkspaceToUserRow is a workspace_to_user row for sync.
type WorkspaceToUserRow struct {
	ID          string     `json:"id"`
	WorkspaceID string     `json:"workspace_id"`
	UserID      string     `json:"user_id"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
}

// RoleRow is a role row for sync.
type RoleRow struct {
	ID          string     `json:"id"`
	WorkspaceID string     `json:"workspace_id"`
	Name        string     `json:"name"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
}

// UserToRoleRow is a user_to_role row for sync.
type UserToRoleRow struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id"`
	RoleID      string     `json:"role_id"`
	WorkspaceID string     `json:"workspace_id"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
}

// PermissionRow is a permission row for sync.
type PermissionRow struct {
	ID           string     `json:"id"`
	RoleID       string     `json:"role_id"`
	WorkspaceID  string     `json:"workspace_id"`
	DbInstanceID *string    `json:"db_instance_id,omitempty"`
	SchemaName   *string    `json:"schema_name,omitempty"`
	TableName    *string    `json:"table_name,omitempty"`
	ColumnName   *string    `json:"column_name,omitempty"`
	Action       string     `json:"action"`
	Effect       string     `json:"effect"`
	UpdatedAt    time.Time  `json:"updated_at"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty"`
}
