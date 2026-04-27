package sync

import "encoding/json"

type EntityType string

const (
	EntityTypeProject EntityType = "project"
	EntityTypePlan    EntityType = "plan"
	EntityTypeRoom    EntityType = "room"
	EntityTypeWall    EntityType = "wall"
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
	Address     *string `json:"address,omitempty"`
	Description *string `json:"description,omitempty"`
	IsArchived  bool    `json:"isArchived"`
	IsFavourite bool    `json:"isFavourite"`
}

type PlanPayload struct {
	ProjectID   string          `json:"projectId"`
	Name        string          `json:"name"`
	PayloadJSON json.RawMessage `json:"payloadJson,omitempty"`
}

type RoomPayload struct {
	PlanID string  `json:"planId"`
	Name   *string `json:"name,omitempty"`
}

type WallPayload struct {
	RoomID string `json:"roomId"`
}

type PhotoPayload struct {
	ParentType  EntityType `json:"parentType"`
	ParentID    string     `json:"parentId"`
	ContentType string     `json:"contentType"`
	Name        *string    `json:"name,omitempty"`
	Caption     *string    `json:"caption,omitempty"`
	TakenAt     *int64     `json:"takenAt,omitempty"`
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
