package sync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/belchch/rms_platform/api/internal/db"
	synctypes "github.com/belchch/rms_platform/api/internal/sync"
)

// errUnsupportedParentType is returned by photoParentWorkspace when the parentType
// value is not a recognized entity type. Callers use errors.Is to distinguish
// client validation failures from internal DB errors.
var errUnsupportedParentType = errors.New("unsupported parentType")

func workspaceOfPhoto(ctx context.Context, q *db.Queries, photoID string) (string, error) {
	ph, err := q.GetPhotoByID(ctx, photoID)
	if err != nil {
		return "", err
	}
	pa, err := q.GetPhotoableByID(ctx, ph.PhotoableID)
	if err != nil {
		return "", err
	}
	return workspaceFromPhotoableOwner(ctx, q, pa.OwnerType, pa.OwnerID)
}

func workspaceFromPhotoableOwner(ctx context.Context, q *db.Queries, ownerType, ownerID string) (string, error) {
	switch ownerType {
	case "project":
		p, err := q.GetProjectByID(ctx, ownerID)
		if err != nil {
			return "", err
		}
		return p.WorkspaceID, nil
	case "room":
		return workspaceOfRoom(ctx, q, ownerID)
	case "wall":
		return workspaceOfWall(ctx, q, ownerID)
	default:
		return "", fmt.Errorf("owner_type")
	}
}

func photoSnapshot(ctx context.Context, q *db.Queries, p db.Photo) (synctypes.EntitySnapshot, error) {
	pa, err := q.GetPhotoableByID(ctx, p.PhotoableID)
	if err != nil {
		return synctypes.EntitySnapshot{}, err
	}
	pl := synctypes.PhotoPayload{
		ParentType:  synctypes.EntityType(pa.OwnerType),
		ParentID:    pa.OwnerID,
		ContentType: p.ContentType,
		Name:        p.Name,
		Caption:     p.Caption,
	}
	if p.TakenAt.Valid {
		ms := p.TakenAt.Time.UnixMilli()
		pl.TakenAt = &ms
	}
	raw, err := json.Marshal(pl)
	if err != nil {
		return synctypes.EntitySnapshot{}, err
	}
	return synctypes.EntitySnapshot{
		EntityType: synctypes.EntityTypePhoto,
		EntityID:   p.ID,
		Payload:    raw,
	}, nil
}

func (h *handler) pushPhoto(ctx context.Context, q *db.Queries, wsID string, op synctypes.PushOperation) pushStepResult {
	switch op.Op {
	case synctypes.OpDelete:
		return h.pushPhotoDelete(ctx, q, wsID, op)
	case synctypes.OpCreate, synctypes.OpUpdate:
		return h.pushPhotoUpsert(ctx, q, wsID, op)
	default:
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: "unsupported op"}}
	}
}

func photoParentWorkspace(ctx context.Context, q *db.Queries, p synctypes.PhotoPayload) (string, error) {
	switch p.ParentType {
	case synctypes.EntityTypeProject:
		row, err := q.GetProjectByID(ctx, p.ParentID)
		if err != nil {
			return "", err
		}
		return row.WorkspaceID, nil
	case synctypes.EntityTypeRoom:
		return workspaceOfRoom(ctx, q, p.ParentID)
	case synctypes.EntityTypeWall:
		return workspaceOfWall(ctx, q, p.ParentID)
	default:
		return "", fmt.Errorf("%w: %s", errUnsupportedParentType, p.ParentType)
	}
}

func (h *handler) pushPhotoUpsert(ctx context.Context, q *db.Queries, wsID string, op synctypes.PushOperation) pushStepResult {
	var payload synctypes.PhotoPayload
	if err := json.Unmarshal(op.Payload, &payload); err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "validation", Message: "invalid photo payload"}}
	}
	if payload.ContentType == "" || payload.ParentID == "" {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "validation", Message: "contentType and parentId are required"}}
	}

	pws, err := photoParentWorkspace(ctx, q, payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "notFound", Message: "parent not found"}}
	}
	if errors.Is(err, errUnsupportedParentType) {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "validation", Message: err.Error()}}
	}
	if err != nil {
		return internalPushErr(op, err)
	}
	if ve := validateWorkspace(pws, wsID); ve != nil {
		return pushStepResult{pushError: ve}
	}

	var takenAt pgtype.Timestamptz
	if payload.TakenAt != nil {
		takenAt = epochMsToTimestamptz(*payload.TakenAt)
	}

	row, err := q.GetPhotoByID(ctx, op.EntityID)
	if errors.Is(err, pgx.ErrNoRows) {
		if op.Op == synctypes.OpUpdate {
			return pushStepResult{pushError: &synctypes.PushError{Reason: "notFound", Message: "photo not found"}}
		}
		pa, err := q.UpsertPhotoableByOwner(ctx, db.UpsertPhotoableByOwnerParams{
			ID:        uuid.New().String(),
			OwnerType: string(payload.ParentType),
			OwnerID:   payload.ParentID,
		})
		if err != nil {
			return internalPushErr(op, err)
		}
		out, err := q.UpsertPhoto(ctx, db.UpsertPhotoParams{
			ID:          op.EntityID,
			PhotoableID: pa.ID,
			ContentType: payload.ContentType,
			Name:        payload.Name,
			Caption:     payload.Caption,
			TakenAt:     takenAt,
			UpdatedAt:   epochMsToTimestamptz(op.ClientUpdatedAt),
		})
		if err != nil {
			return internalPushErr(op, err)
		}
		return pushStepResult{applied: true, cursor: out.SyncCursor}
	}
	if err != nil {
		return internalPushErr(op, err)
	}

	phWS, err := workspaceOfPhoto(ctx, q, row.ID)
	if err != nil {
		return internalPushErr(op, err)
	}
	if ve := validateWorkspace(phWS, wsID); ve != nil {
		return pushStepResult{pushError: ve}
	}

	existingPA, err := q.GetPhotoableByID(ctx, row.PhotoableID)
	if err != nil {
		return internalPushErr(op, err)
	}
	if string(payload.ParentType) != existingPA.OwnerType || payload.ParentID != existingPA.OwnerID {
		snap, err := photoSnapshot(ctx, q, row)
		if err != nil {
			return internalPushErr(op, err)
		}
		return pushStepResult{conflict: &synctypes.PushConflict{Reason: "parentMismatch", ServerVersion: snap}}
	}

	if !lwwWins(op.ClientUpdatedAt, row.UpdatedAt) {
		snap, err := photoSnapshot(ctx, q, row)
		if err != nil {
			return internalPushErr(op, err)
		}
		return pushStepResult{conflict: &synctypes.PushConflict{Reason: "stale", ServerVersion: snap}}
	}

	out, err := q.UpsertPhoto(ctx, db.UpsertPhotoParams{
		ID:          op.EntityID,
		PhotoableID: row.PhotoableID,
		ContentType: payload.ContentType,
		Name:        payload.Name,
		Caption:     payload.Caption,
		TakenAt:     takenAt,
		UpdatedAt:   epochMsToTimestamptz(op.ClientUpdatedAt),
	})
	if err != nil {
		return internalPushErr(op, err)
	}
	return pushStepResult{applied: true, cursor: out.SyncCursor}
}

func (h *handler) pushPhotoDelete(ctx context.Context, q *db.Queries, wsID string, op synctypes.PushOperation) pushStepResult {
	row, err := q.GetPhotoByID(ctx, op.EntityID)
	if errors.Is(err, pgx.ErrNoRows) {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "notFound", Message: "photo not found"}}
	}
	if err != nil {
		return internalPushErr(op, err)
	}
	phWS, err := workspaceOfPhoto(ctx, q, row.ID)
	if err != nil {
		return internalPushErr(op, err)
	}
	if ve := validateWorkspace(phWS, wsID); ve != nil {
		return pushStepResult{pushError: ve}
	}
	if !lwwWins(op.ClientUpdatedAt, row.UpdatedAt) {
		snap, err := photoSnapshot(ctx, q, row)
		if err != nil {
			return internalPushErr(op, err)
		}
		return pushStepResult{conflict: &synctypes.PushConflict{Reason: "stale", ServerVersion: snap}}
	}
	out, err := q.SoftDeletePhoto(ctx, db.SoftDeletePhotoParams{
		ID:        op.EntityID,
		UpdatedAt: epochMsToTimestamptz(op.ClientUpdatedAt),
	})
	if err != nil {
		return internalPushErr(op, err)
	}
	return pushStepResult{applied: true, cursor: out.SyncCursor}
}
