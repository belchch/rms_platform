package sync

import "encoding/json"

type EntityType string

const (
	EntityTypeProject EntityType = "project"
	EntityTypePlan    EntityType = "plan"
	EntityTypePhoto   EntityType = "photo"
)

type OpType string

const (
	OpCreate OpType = "create"
	OpUpdate OpType = "update"
	OpDelete OpType = "delete"
)

type ProjectPayload struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

type PlanPayload struct {
	ProjectID   string  `json:"projectId"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

type PhotoPayload struct {
	PlanID      string `json:"planId"`
	ContentType string `json:"contentType"`
}

type EntitySnapshot struct {
	EntityType EntityType      `json:"entityType"`
	EntityID   string          `json:"entityId"`
	Payload    json.RawMessage `json:"payload"`
}

type PushOperation struct {
	ClientOpID      string          `json:"clientOpId"`
	Op              OpType          `json:"op"`
	EntityType      EntityType      `json:"entityType"`
	EntityID        string          `json:"entityId"`
	ClientUpdatedAt int64           `json:"clientUpdatedAt"`
	Payload         json.RawMessage `json:"payload"`
}

type PullChange struct {
	EntityType EntityType      `json:"entityType"`
	EntityID   string          `json:"entityId"`
	Payload    json.RawMessage `json:"payload"`
	UpdatedAt  int64           `json:"updatedAt"`
	SyncCursor int64           `json:"syncCursor"`
	DeletedAt  *int64          `json:"deletedAt,omitempty"`
}

type PushConflict struct {
	ClientOpID    string         `json:"clientOpId"`
	Reason        string         `json:"reason"`
	ServerVersion EntitySnapshot `json:"serverVersion"`
}

type PushError struct {
	ClientOpID string `json:"clientOpId"`
	Reason     string `json:"reason"`
	Message    string `json:"message"`
}
