package photo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"

	"github.com/belchch/rms_platform/api/internal/db"
	"github.com/belchch/rms_platform/api/internal/handler/sync/syncdomain"
	synctypes "github.com/belchch/rms_platform/api/internal/sync"
)

var errUnsupportedParentType = errors.New("unsupported parentType")

// errUnsupportedOwnerType signals a storage integrity violation: owner_type was persisted
// but is no longer recognized. Distinct from errUnsupportedParentType (client mistake).
var errUnsupportedOwnerType = errors.New("unsupported owner_type in storage")

func workspaceOfPhoto(ctx context.Context, q db.Querier, photoID string) (string, error) {
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

func workspaceFromPhotoableOwner(ctx context.Context, q db.Querier, ownerType, ownerID string) (string, error) {
	switch ownerType {
	case "project":
		p, err := q.GetProjectByID(ctx, ownerID)
		if err != nil {
			return "", err
		}
		return p.WorkspaceID, nil
	case "room":
		return syncdomain.WorkspaceOfRoom(ctx, q, ownerID)
	case "wall":
		return syncdomain.WorkspaceOfWall(ctx, q, ownerID)
	default:
		return "", fmt.Errorf("%w: %s", errUnsupportedOwnerType, ownerType)
	}
}

func entityTypeFromPhotoOwner(ownerType string) (synctypes.EntityType, error) {
	switch ownerType {
	case "project":
		return synctypes.EntityTypeProject, nil
	case "room":
		return synctypes.EntityTypeRoom, nil
	case "wall":
		return synctypes.EntityTypeWall, nil
	default:
		return "", fmt.Errorf("%w: %s", errUnsupportedOwnerType, ownerType)
	}
}

func listPhotosSinceRowToPhoto(r db.ListPhotosSinceRow) db.Photo {
	return db.Photo{
		ID:          r.ID,
		PhotoableID: r.PhotoableID,
		RemoteUrl:   r.RemoteUrl,
		ContentType: r.ContentType,
		Name:        r.Name,
		Caption:     r.Caption,
		TakenAt:     r.TakenAt,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
		DeletedAt:   r.DeletedAt,
		SyncCursor:  r.SyncCursor,
	}
}

func photoSnapshotFromOwnerAndPhoto(ownerType, ownerID string, p db.Photo) (synctypes.EntitySnapshot, error) {
	pt, err := entityTypeFromPhotoOwner(ownerType)
	if err != nil {
		return synctypes.EntitySnapshot{}, err
	}
	pl := synctypes.PhotoPayload{
		ParentType:  pt,
		ParentID:    ownerID,
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

func photoSnapshot(ctx context.Context, q db.Querier, p db.Photo) (synctypes.EntitySnapshot, error) {
	pa, err := q.GetPhotoableByID(ctx, p.PhotoableID)
	if err != nil {
		return synctypes.EntitySnapshot{}, err
	}
	return photoSnapshotFromOwnerAndPhoto(pa.OwnerType, pa.OwnerID, p)
}

func photoSnapshotFromPullRow(r db.ListPhotosSinceRow) (synctypes.EntitySnapshot, error) {
	return photoSnapshotFromOwnerAndPhoto(r.OwnerType, r.OwnerID, listPhotosSinceRowToPhoto(r))
}

func ApplyPush(ctx context.Context, q db.Querier, wsID string, op synctypes.PushOperation) syncdomain.PushStepResult {
	switch op.Op {
	case synctypes.OpDelete:
		return pushDelete(ctx, q, wsID, op)
	case synctypes.OpCreate, synctypes.OpUpdate:
		return pushUpsert(ctx, q, wsID, op)
	default:
		return syncdomain.PushStepResult{PushError: &synctypes.PushError{Reason: "unknown", Message: "unsupported op"}}
	}
}

func photoParentWorkspace(ctx context.Context, q db.Querier, p synctypes.PhotoPayload) (string, error) {
	switch p.ParentType {
	case synctypes.EntityTypeProject:
		row, err := q.GetProjectByID(ctx, p.ParentID)
		if err != nil {
			return "", err
		}
		return row.WorkspaceID, nil
	case synctypes.EntityTypeRoom:
		return syncdomain.WorkspaceOfRoom(ctx, q, p.ParentID)
	case synctypes.EntityTypeWall:
		return syncdomain.WorkspaceOfWall(ctx, q, p.ParentID)
	default:
		return "", fmt.Errorf("%w: %s", errUnsupportedParentType, p.ParentType)
	}
}

func pushUpsert(ctx context.Context, q db.Querier, wsID string, op synctypes.PushOperation) syncdomain.PushStepResult {
	var payload synctypes.PhotoPayload
	if err := json.Unmarshal(op.Payload, &payload); err != nil {
		return syncdomain.PushStepResult{PushError: &synctypes.PushError{Reason: "validation", Message: "invalid photo payload"}}
	}
	if payload.ContentType == "" || payload.ParentID == "" {
		return syncdomain.PushStepResult{PushError: &synctypes.PushError{Reason: "validation", Message: "contentType and parentId are required"}}
	}

	pws, err := photoParentWorkspace(ctx, q, payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return syncdomain.PushStepResult{PushError: &synctypes.PushError{Reason: "notFound", Message: "parent not found"}}
	}
	if errors.Is(err, errUnsupportedParentType) {
		return syncdomain.PushStepResult{PushError: &synctypes.PushError{Reason: "validation", Message: err.Error()}}
	}
	if err != nil {
		return syncdomain.InternalPushErr(op, err)
	}
	if ve := syncdomain.ValidateWorkspace(pws, wsID); ve != nil {
		return syncdomain.PushStepResult{PushError: ve}
	}

	var takenAt pgtype.Timestamptz
	if payload.TakenAt != nil {
		takenAt = syncdomain.EpochMsToTimestamptz(*payload.TakenAt)
	}

	row, err := q.GetPhotoByID(ctx, op.EntityID)
	if errors.Is(err, pgx.ErrNoRows) {
		if op.Op == synctypes.OpUpdate {
			return syncdomain.PushStepResult{PushError: &synctypes.PushError{Reason: "notFound", Message: "photo not found"}}
		}
		pa, err := q.UpsertPhotoableByOwner(ctx, db.UpsertPhotoableByOwnerParams{
			ID:        uuid.New().String(),
			OwnerType: string(payload.ParentType),
			OwnerID:   payload.ParentID,
		})
		if err != nil {
			return syncdomain.InternalPushErr(op, err)
		}
		out, err := q.UpsertPhoto(ctx, db.UpsertPhotoParams{
			ID:          op.EntityID,
			PhotoableID: pa.ID,
			ContentType: payload.ContentType,
			Name:        payload.Name,
			Caption:     payload.Caption,
			TakenAt:     takenAt,
			UpdatedAt:   syncdomain.EpochMsToTimestamptz(op.ClientUpdatedAt),
		})
		if err != nil {
			return syncdomain.InternalPushErr(op, err)
		}
		return syncdomain.PushStepResult{Applied: true, Cursor: out.SyncCursor}
	}
	if err != nil {
		return syncdomain.InternalPushErr(op, err)
	}

	phWS, err := workspaceOfPhoto(ctx, q, row.ID)
	if errors.Is(err, errUnsupportedOwnerType) {
		log.Error().Err(err).Str("entityId", op.EntityID).Msg("photo owner_type invariant violated")
		return syncdomain.PushStepResult{PushError: &synctypes.PushError{Reason: "dataIntegrity", Message: "photo parent type is not recognized"}}
	}
	if err != nil {
		return syncdomain.InternalPushErr(op, err)
	}
	if ve := syncdomain.ValidateWorkspace(phWS, wsID); ve != nil {
		return syncdomain.PushStepResult{PushError: ve}
	}

	existingPA, err := q.GetPhotoableByID(ctx, row.PhotoableID)
	if err != nil {
		return syncdomain.InternalPushErr(op, err)
	}
	if string(payload.ParentType) != existingPA.OwnerType || payload.ParentID != existingPA.OwnerID {
		snap, err := photoSnapshot(ctx, q, row)
		if err != nil {
			return syncdomain.InternalPushErr(op, err)
		}
		return syncdomain.PushStepResult{Conflict: &synctypes.PushConflict{Reason: "parentMismatch", ServerVersion: snap}}
	}

	wins, err := syncdomain.LWWWins(op.ClientUpdatedAt, row.UpdatedAt)
	if err != nil {
		return syncdomain.InternalPushErr(op, err)
	}
	if !wins {
		snap, err := photoSnapshot(ctx, q, row)
		if err != nil {
			return syncdomain.InternalPushErr(op, err)
		}
		return syncdomain.PushStepResult{Conflict: &synctypes.PushConflict{Reason: "stale", ServerVersion: snap}}
	}

	out, err := q.UpsertPhoto(ctx, db.UpsertPhotoParams{
		ID:          op.EntityID,
		PhotoableID: row.PhotoableID,
		ContentType: payload.ContentType,
		Name:        payload.Name,
		Caption:     payload.Caption,
		TakenAt:     takenAt,
		UpdatedAt:   syncdomain.EpochMsToTimestamptz(op.ClientUpdatedAt),
	})
	if err != nil {
		return syncdomain.InternalPushErr(op, err)
	}
	return syncdomain.PushStepResult{Applied: true, Cursor: out.SyncCursor}
}

func pushDelete(ctx context.Context, q db.Querier, wsID string, op synctypes.PushOperation) syncdomain.PushStepResult {
	row, err := q.GetPhotoByID(ctx, op.EntityID)
	if errors.Is(err, pgx.ErrNoRows) {
		return syncdomain.PushStepResult{PushError: &synctypes.PushError{Reason: "notFound", Message: "photo not found"}}
	}
	if err != nil {
		return syncdomain.InternalPushErr(op, err)
	}
	phWS, err := workspaceOfPhoto(ctx, q, row.ID)
	if errors.Is(err, errUnsupportedOwnerType) {
		log.Error().Err(err).Str("entityId", op.EntityID).Msg("photo owner_type invariant violated")
		return syncdomain.PushStepResult{PushError: &synctypes.PushError{Reason: "dataIntegrity", Message: "photo parent type is not recognized"}}
	}
	if err != nil {
		return syncdomain.InternalPushErr(op, err)
	}
	if ve := syncdomain.ValidateWorkspace(phWS, wsID); ve != nil {
		return syncdomain.PushStepResult{PushError: ve}
	}
	wins, err := syncdomain.LWWWins(op.ClientUpdatedAt, row.UpdatedAt)
	if err != nil {
		return syncdomain.InternalPushErr(op, err)
	}
	if !wins {
		snap, err := photoSnapshot(ctx, q, row)
		if err != nil {
			return syncdomain.InternalPushErr(op, err)
		}
		return syncdomain.PushStepResult{Conflict: &synctypes.PushConflict{Reason: "stale", ServerVersion: snap}}
	}
	out, err := q.SoftDeletePhoto(ctx, db.SoftDeletePhotoParams{
		ID:        op.EntityID,
		UpdatedAt: syncdomain.EpochMsToTimestamptz(op.ClientUpdatedAt),
	})
	if err != nil {
		return syncdomain.InternalPushErr(op, err)
	}
	return syncdomain.PushStepResult{Applied: true, Cursor: out.SyncCursor}
}

func AppendPullChanges(changes []synctypes.PullChange, rows []db.ListPhotosSinceRow) ([]synctypes.PullChange, error) {
	return syncdomain.AppendPullChangesFromRows(changes, rows, func(r db.ListPhotosSinceRow) (synctypes.PullChange, error) {
		snap, err := photoSnapshotFromPullRow(r)
		if err != nil {
			log.Error().Err(err).Str("entityType", string(synctypes.EntityTypePhoto)).Str("entityId", r.ID).Msg("sync pull snapshot failed")
			return synctypes.PullChange{}, fmt.Errorf("sync pull photo snapshot: %w", err)
		}
		pc, err := syncdomain.PullChangeFromSnapshot(snap, r.UpdatedAt, r.SyncCursor, r.DeletedAt)
		if err != nil {
			log.Error().Err(err).Str("entityType", string(synctypes.EntityTypePhoto)).Str("entityId", r.ID).Msg("sync pull snapshot failed")
			return synctypes.PullChange{}, fmt.Errorf("sync pull photo snapshot: %w", err)
		}
		return pc, nil
	})
}
