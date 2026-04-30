package sync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/belchch/rms_platform/api/internal/db"
	mid "github.com/belchch/rms_platform/api/internal/middleware"
	synctypes "github.com/belchch/rms_platform/api/internal/sync"
)

type handler struct {
	pool *pgxpool.Pool
}

func pgTimeToEpochMs(t pgtype.Timestamptz) int64 {
	if !t.Valid {
		return 0
	}
	return t.Time.UnixMilli()
}

func epochMsToTimestamptz(ms int64) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: time.UnixMilli(ms), Valid: true}
}

func lwwWins(clientMs int64, serverUpdated pgtype.Timestamptz) bool {
	return clientMs > pgTimeToEpochMs(serverUpdated)
}

func workspaceOfPlan(ctx context.Context, q *db.Queries, planID string) (string, error) {
	pl, err := q.GetPlanByID(ctx, planID)
	if err != nil {
		return "", err
	}
	p, err := q.GetProjectByID(ctx, pl.ProjectID)
	if err != nil {
		return "", err
	}
	return p.WorkspaceID, nil
}

func workspaceOfRoom(ctx context.Context, q *db.Queries, roomID string) (string, error) {
	r, err := q.GetRoomByID(ctx, roomID)
	if err != nil {
		return "", err
	}
	return workspaceOfPlan(ctx, q, r.PlanID)
}

func workspaceOfWall(ctx context.Context, q *db.Queries, wallID string) (string, error) {
	w, err := q.GetWallByID(ctx, wallID)
	if err != nil {
		return "", err
	}
	return workspaceOfRoom(ctx, q, w.RoomID)
}

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

func validateWorkspace(actualWS, jwtWS string) *synctypes.PushError {
	if actualWS != jwtWS {
		return &synctypes.PushError{Reason: "forbidden", Message: "entity belongs to another workspace"}
	}
	return nil
}

func projectSnapshot(p db.Project) (synctypes.EntitySnapshot, error) {
	pl := synctypes.ProjectPayload{
		Name:        p.Name,
		Address:     p.Address,
		Description: p.Description,
		IsArchived:  p.IsArchived,
		IsFavourite: p.IsFavourite,
	}
	raw, err := json.Marshal(pl)
	if err != nil {
		return synctypes.EntitySnapshot{}, err
	}
	return synctypes.EntitySnapshot{
		EntityType: synctypes.EntityTypeProject,
		EntityID:   p.ID,
		Payload:    raw,
	}, nil
}

func planSnapshot(p db.Plan) (synctypes.EntitySnapshot, error) {
	pl := synctypes.PlanPayload{
		ProjectID:   p.ProjectID,
		Name:        p.Name,
		PayloadJSON: p.PayloadJson,
	}
	raw, err := json.Marshal(pl)
	if err != nil {
		return synctypes.EntitySnapshot{}, err
	}
	return synctypes.EntitySnapshot{
		EntityType: synctypes.EntityTypePlan,
		EntityID:   p.ID,
		Payload:    raw,
	}, nil
}

func roomSnapshot(r db.Room) (synctypes.EntitySnapshot, error) {
	pl := synctypes.RoomPayload{
		PlanID: r.PlanID,
		Name:   r.Name,
	}
	raw, err := json.Marshal(pl)
	if err != nil {
		return synctypes.EntitySnapshot{}, err
	}
	return synctypes.EntitySnapshot{
		EntityType: synctypes.EntityTypeRoom,
		EntityID:   r.ID,
		Payload:    raw,
	}, nil
}

func wallSnapshot(w db.Wall) (synctypes.EntitySnapshot, error) {
	pl := synctypes.WallPayload{RoomID: w.RoomID}
	raw, err := json.Marshal(pl)
	if err != nil {
		return synctypes.EntitySnapshot{}, err
	}
	return synctypes.EntitySnapshot{
		EntityType: synctypes.EntityTypeWall,
		EntityID:   w.ID,
		Payload:    raw,
	}, nil
}

func photoSnapshot(ctx context.Context, q *db.Queries, p db.Photo) (synctypes.EntitySnapshot, error) {
	pa, err := q.GetPhotoableByID(ctx, p.PhotoableID)
	if err != nil {
		return synctypes.EntitySnapshot{}, err
	}
	pl := synctypes.PhotoPayload{
		ParentType: synctypes.EntityType(pa.OwnerType),
		ParentID:   pa.OwnerID,
		ContentType: "",
		Name:       p.Name,
		Caption:    p.Caption,
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

type pushStepResult struct {
	applied   bool
	conflict  *synctypes.PushConflict
	pushError *synctypes.PushError
	cursor    int64
}

func (h *handler) push(ctx context.Context, in *PushInput) (*PushOutput, error) {
	wsID, ok := mid.WorkspaceID(ctx)
	if !ok {
		return nil, huma.NewError(http.StatusUnauthorized, "Unauthorized")
	}

	out := &PushOutput{}
	out.Body.Applied = []string{}
	out.Body.Conflicts = []synctypes.PushConflict{}
	out.Body.Errors = []synctypes.PushError{}
	out.Body.Cursor = 0

	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("sync push begin: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := db.New(tx)
	var maxCursor int64

	for i, op := range in.Body.Operations {
		sp := "sp_" + strconv.Itoa(i)
		if _, err := tx.Exec(ctx, "SAVEPOINT "+sp); err != nil {
			return nil, fmt.Errorf("sync push savepoint: %w", err)
		}

		res := h.applyPushOperation(ctx, qtx, wsID, op)

		if res.pushError != nil {
			res.pushError.ClientOpID = op.ClientOpID
			out.Body.Errors = append(out.Body.Errors, *res.pushError)
			if _, rbErr := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT "+sp); rbErr != nil {
				return nil, fmt.Errorf("sync push rollback savepoint: %w", rbErr)
			}
			continue
		}
		if res.conflict != nil {
			res.conflict.ClientOpID = op.ClientOpID
			out.Body.Conflicts = append(out.Body.Conflicts, *res.conflict)
			if _, rbErr := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT "+sp); rbErr != nil {
				return nil, fmt.Errorf("sync push rollback savepoint: %w", rbErr)
			}
			continue
		}
		if res.applied {
			out.Body.Applied = append(out.Body.Applied, op.ClientOpID)
			if res.cursor > maxCursor {
				maxCursor = res.cursor
			}
		}
		if _, err := tx.Exec(ctx, "RELEASE SAVEPOINT "+sp); err != nil {
			return nil, fmt.Errorf("sync push release savepoint: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("sync push commit: %w", err)
	}

	out.Body.Cursor = maxCursor
	return out, nil
}

func (h *handler) applyPushOperation(ctx context.Context, q *db.Queries, wsID string, op synctypes.PushOperation) pushStepResult {
	switch op.EntityType {
	case synctypes.EntityTypeProject:
		return h.pushProject(ctx, q, wsID, op)
	case synctypes.EntityTypePlan:
		return h.pushPlan(ctx, q, wsID, op)
	case synctypes.EntityTypeRoom:
		return h.pushRoom(ctx, q, wsID, op)
	case synctypes.EntityTypeWall:
		return h.pushWall(ctx, q, wsID, op)
	case synctypes.EntityTypePhoto:
		return h.pushPhoto(ctx, q, wsID, op)
	default:
		return pushStepResult{
			pushError: &synctypes.PushError{Reason: "unknown", Message: "unsupported entityType"},
		}
	}
}

func (h *handler) pushProject(ctx context.Context, q *db.Queries, wsID string, op synctypes.PushOperation) pushStepResult {
	switch op.Op {
	case synctypes.OpDelete:
		return h.pushProjectDelete(ctx, q, wsID, op)
	case synctypes.OpCreate, synctypes.OpUpdate:
		return h.pushProjectUpsert(ctx, q, wsID, op)
	default:
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: "unsupported op"}}
	}
}

func (h *handler) pushProjectUpsert(ctx context.Context, q *db.Queries, wsID string, op synctypes.PushOperation) pushStepResult {
	var payload synctypes.ProjectPayload
	if err := json.Unmarshal(op.Payload, &payload); err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "validation", Message: "invalid project payload"}}
	}
	if payload.Name == "" {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "validation", Message: "name is required"}}
	}

	row, err := q.GetProjectByID(ctx, op.EntityID)
	if errors.Is(err, pgx.ErrNoRows) {
		if op.Op == synctypes.OpUpdate {
			return pushStepResult{pushError: &synctypes.PushError{Reason: "notFound", Message: "project not found"}}
		}
		out, err := q.UpsertProject(ctx, db.UpsertProjectParams{
			ID:          op.EntityID,
			WorkspaceID: wsID,
			Name:        payload.Name,
			Address:     payload.Address,
			Description: payload.Description,
			IsArchived:  payload.IsArchived,
			IsFavourite: payload.IsFavourite,
			UpdatedAt:   epochMsToTimestamptz(op.ClientUpdatedAt),
		})
		if err != nil {
			return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
		}
		return pushStepResult{applied: true, cursor: out.SyncCursor}
	}
	if err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
	}
	if ve := validateWorkspace(row.WorkspaceID, wsID); ve != nil {
		return pushStepResult{pushError: ve}
	}
	if !lwwWins(op.ClientUpdatedAt, row.UpdatedAt) {
		snap, err := projectSnapshot(row)
		if err != nil {
			return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
		}
		return pushStepResult{conflict: &synctypes.PushConflict{Reason: "stale", ServerVersion: snap}}
	}
	out, err := q.UpsertProject(ctx, db.UpsertProjectParams{
		ID:          op.EntityID,
		WorkspaceID: wsID,
		Name:        payload.Name,
		Address:     payload.Address,
		Description: payload.Description,
		IsArchived:  payload.IsArchived,
		IsFavourite: payload.IsFavourite,
		UpdatedAt:   epochMsToTimestamptz(op.ClientUpdatedAt),
	})
	if err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
	}
	return pushStepResult{applied: true, cursor: out.SyncCursor}
}

func (h *handler) pushProjectDelete(ctx context.Context, q *db.Queries, wsID string, op synctypes.PushOperation) pushStepResult {
	row, err := q.GetProjectByID(ctx, op.EntityID)
	if errors.Is(err, pgx.ErrNoRows) {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "notFound", Message: "project not found"}}
	}
	if err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
	}
	if ve := validateWorkspace(row.WorkspaceID, wsID); ve != nil {
		return pushStepResult{pushError: ve}
	}
	if !lwwWins(op.ClientUpdatedAt, row.UpdatedAt) {
		snap, err := projectSnapshot(row)
		if err != nil {
			return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
		}
		return pushStepResult{conflict: &synctypes.PushConflict{Reason: "stale", ServerVersion: snap}}
	}
	out, err := q.SoftDeleteProject(ctx, db.SoftDeleteProjectParams{
		ID:        op.EntityID,
		UpdatedAt: epochMsToTimestamptz(op.ClientUpdatedAt),
	})
	if err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
	}
	return pushStepResult{applied: true, cursor: out.SyncCursor}
}

func (h *handler) pushPlan(ctx context.Context, q *db.Queries, wsID string, op synctypes.PushOperation) pushStepResult {
	switch op.Op {
	case synctypes.OpDelete:
		return h.pushPlanDelete(ctx, q, wsID, op)
	case synctypes.OpCreate, synctypes.OpUpdate:
		return h.pushPlanUpsert(ctx, q, wsID, op)
	default:
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: "unsupported op"}}
	}
}

func (h *handler) pushPlanUpsert(ctx context.Context, q *db.Queries, wsID string, op synctypes.PushOperation) pushStepResult {
	var payload synctypes.PlanPayload
	if err := json.Unmarshal(op.Payload, &payload); err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "validation", Message: "invalid plan payload"}}
	}
	if payload.ProjectID == "" || payload.Name == "" {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "validation", Message: "projectId and name are required"}}
	}

	parentProj, err := q.GetProjectByID(ctx, payload.ProjectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "notFound", Message: "project not found"}}
	}
	if err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
	}
	if ve := validateWorkspace(parentProj.WorkspaceID, wsID); ve != nil {
		return pushStepResult{pushError: ve}
	}

	row, err := q.GetPlanByID(ctx, op.EntityID)
	if errors.Is(err, pgx.ErrNoRows) {
		if op.Op == synctypes.OpUpdate {
			return pushStepResult{pushError: &synctypes.PushError{Reason: "notFound", Message: "plan not found"}}
		}
		out, err := q.UpsertPlan(ctx, db.UpsertPlanParams{
			ID:          op.EntityID,
			ProjectID:   payload.ProjectID,
			Name:        payload.Name,
			PayloadJson: payload.PayloadJSON,
			UpdatedAt:   epochMsToTimestamptz(op.ClientUpdatedAt),
		})
		if err != nil {
			return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
		}
		return pushStepResult{applied: true, cursor: out.SyncCursor}
	}
	if err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
	}

	pws, err := workspaceOfPlan(ctx, q, row.ID)
	if err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
	}
	if ve := validateWorkspace(pws, wsID); ve != nil {
		return pushStepResult{pushError: ve}
	}

	if !lwwWins(op.ClientUpdatedAt, row.UpdatedAt) {
		snap, err := planSnapshot(row)
		if err != nil {
			return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
		}
		return pushStepResult{conflict: &synctypes.PushConflict{Reason: "stale", ServerVersion: snap}}
	}

	out, err := q.UpsertPlan(ctx, db.UpsertPlanParams{
		ID:          op.EntityID,
		ProjectID:   payload.ProjectID,
		Name:        payload.Name,
		PayloadJson: payload.PayloadJSON,
		UpdatedAt:   epochMsToTimestamptz(op.ClientUpdatedAt),
	})
	if err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
	}
	return pushStepResult{applied: true, cursor: out.SyncCursor}
}

func (h *handler) pushPlanDelete(ctx context.Context, q *db.Queries, wsID string, op synctypes.PushOperation) pushStepResult {
	row, err := q.GetPlanByID(ctx, op.EntityID)
	if errors.Is(err, pgx.ErrNoRows) {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "notFound", Message: "plan not found"}}
	}
	if err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
	}
	pws, err := workspaceOfPlan(ctx, q, row.ID)
	if err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
	}
	if ve := validateWorkspace(pws, wsID); ve != nil {
		return pushStepResult{pushError: ve}
	}
	if !lwwWins(op.ClientUpdatedAt, row.UpdatedAt) {
		snap, err := planSnapshot(row)
		if err != nil {
			return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
		}
		return pushStepResult{conflict: &synctypes.PushConflict{Reason: "stale", ServerVersion: snap}}
	}
	out, err := q.SoftDeletePlan(ctx, db.SoftDeletePlanParams{
		ID:        op.EntityID,
		UpdatedAt: epochMsToTimestamptz(op.ClientUpdatedAt),
	})
	if err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
	}
	return pushStepResult{applied: true, cursor: out.SyncCursor}
}

func (h *handler) pushRoom(ctx context.Context, q *db.Queries, wsID string, op synctypes.PushOperation) pushStepResult {
	switch op.Op {
	case synctypes.OpDelete:
		return h.pushRoomDelete(ctx, q, wsID, op)
	case synctypes.OpCreate, synctypes.OpUpdate:
		return h.pushRoomUpsert(ctx, q, wsID, op)
	default:
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: "unsupported op"}}
	}
}

func (h *handler) pushRoomUpsert(ctx context.Context, q *db.Queries, wsID string, op synctypes.PushOperation) pushStepResult {
	var payload synctypes.RoomPayload
	if err := json.Unmarshal(op.Payload, &payload); err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "validation", Message: "invalid room payload"}}
	}
	if payload.PlanID == "" {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "validation", Message: "planId is required"}}
	}

	planParentWS, err := workspaceOfPlan(ctx, q, payload.PlanID)
	if errors.Is(err, pgx.ErrNoRows) {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "notFound", Message: "plan not found"}}
	}
	if err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
	}
	if ve := validateWorkspace(planParentWS, wsID); ve != nil {
		return pushStepResult{pushError: ve}
	}

	row, err := q.GetRoomByID(ctx, op.EntityID)
	if errors.Is(err, pgx.ErrNoRows) {
		if op.Op == synctypes.OpUpdate {
			return pushStepResult{pushError: &synctypes.PushError{Reason: "notFound", Message: "room not found"}}
		}
		out, err := q.UpsertRoom(ctx, db.UpsertRoomParams{
			ID:        op.EntityID,
			PlanID:    payload.PlanID,
			Name:      payload.Name,
			UpdatedAt: epochMsToTimestamptz(op.ClientUpdatedAt),
		})
		if err != nil {
			return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
		}
		return pushStepResult{applied: true, cursor: out.SyncCursor}
	}
	if err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
	}
	rws, err := workspaceOfRoom(ctx, q, row.ID)
	if err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
	}
	if ve := validateWorkspace(rws, wsID); ve != nil {
		return pushStepResult{pushError: ve}
	}
	if !lwwWins(op.ClientUpdatedAt, row.UpdatedAt) {
		snap, err := roomSnapshot(row)
		if err != nil {
			return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
		}
		return pushStepResult{conflict: &synctypes.PushConflict{Reason: "stale", ServerVersion: snap}}
	}
	out, err := q.UpsertRoom(ctx, db.UpsertRoomParams{
		ID:        op.EntityID,
		PlanID:    payload.PlanID,
		Name:      payload.Name,
		UpdatedAt: epochMsToTimestamptz(op.ClientUpdatedAt),
	})
	if err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
	}
	return pushStepResult{applied: true, cursor: out.SyncCursor}
}

func (h *handler) pushRoomDelete(ctx context.Context, q *db.Queries, wsID string, op synctypes.PushOperation) pushStepResult {
	row, err := q.GetRoomByID(ctx, op.EntityID)
	if errors.Is(err, pgx.ErrNoRows) {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "notFound", Message: "room not found"}}
	}
	if err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
	}
	rws, err := workspaceOfRoom(ctx, q, row.ID)
	if err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
	}
	if ve := validateWorkspace(rws, wsID); ve != nil {
		return pushStepResult{pushError: ve}
	}
	if !lwwWins(op.ClientUpdatedAt, row.UpdatedAt) {
		snap, err := roomSnapshot(row)
		if err != nil {
			return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
		}
		return pushStepResult{conflict: &synctypes.PushConflict{Reason: "stale", ServerVersion: snap}}
	}
	out, err := q.SoftDeleteRoom(ctx, db.SoftDeleteRoomParams{
		ID:        op.EntityID,
		UpdatedAt: epochMsToTimestamptz(op.ClientUpdatedAt),
	})
	if err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
	}
	return pushStepResult{applied: true, cursor: out.SyncCursor}
}

func (h *handler) pushWall(ctx context.Context, q *db.Queries, wsID string, op synctypes.PushOperation) pushStepResult {
	switch op.Op {
	case synctypes.OpDelete:
		return h.pushWallDelete(ctx, q, wsID, op)
	case synctypes.OpCreate, synctypes.OpUpdate:
		return h.pushWallUpsert(ctx, q, wsID, op)
	default:
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: "unsupported op"}}
	}
}

func (h *handler) pushWallUpsert(ctx context.Context, q *db.Queries, wsID string, op synctypes.PushOperation) pushStepResult {
	var payload synctypes.WallPayload
	if err := json.Unmarshal(op.Payload, &payload); err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "validation", Message: "invalid wall payload"}}
	}
	if payload.RoomID == "" {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "validation", Message: "roomId is required"}}
	}

	rws, err := workspaceOfRoom(ctx, q, payload.RoomID)
	if errors.Is(err, pgx.ErrNoRows) {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "notFound", Message: "room not found"}}
	}
	if err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
	}
	if ve := validateWorkspace(rws, wsID); ve != nil {
		return pushStepResult{pushError: ve}
	}

	row, err := q.GetWallByID(ctx, op.EntityID)
	if errors.Is(err, pgx.ErrNoRows) {
		if op.Op == synctypes.OpUpdate {
			return pushStepResult{pushError: &synctypes.PushError{Reason: "notFound", Message: "wall not found"}}
		}
		out, err := q.UpsertWall(ctx, db.UpsertWallParams{
			ID:        op.EntityID,
			RoomID:    payload.RoomID,
			UpdatedAt: epochMsToTimestamptz(op.ClientUpdatedAt),
		})
		if err != nil {
			return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
		}
		return pushStepResult{applied: true, cursor: out.SyncCursor}
	}
	if err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
	}
	wws, err := workspaceOfWall(ctx, q, row.ID)
	if err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
	}
	if ve := validateWorkspace(wws, wsID); ve != nil {
		return pushStepResult{pushError: ve}
	}
	if !lwwWins(op.ClientUpdatedAt, row.UpdatedAt) {
		snap, err := wallSnapshot(row)
		if err != nil {
			return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
		}
		return pushStepResult{conflict: &synctypes.PushConflict{Reason: "stale", ServerVersion: snap}}
	}
	out, err := q.UpsertWall(ctx, db.UpsertWallParams{
		ID:        op.EntityID,
		RoomID:    payload.RoomID,
		UpdatedAt: epochMsToTimestamptz(op.ClientUpdatedAt),
	})
	if err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
	}
	return pushStepResult{applied: true, cursor: out.SyncCursor}
}

func (h *handler) pushWallDelete(ctx context.Context, q *db.Queries, wsID string, op synctypes.PushOperation) pushStepResult {
	row, err := q.GetWallByID(ctx, op.EntityID)
	if errors.Is(err, pgx.ErrNoRows) {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "notFound", Message: "wall not found"}}
	}
	if err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
	}
	wws, err := workspaceOfWall(ctx, q, row.ID)
	if err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
	}
	if ve := validateWorkspace(wws, wsID); ve != nil {
		return pushStepResult{pushError: ve}
	}
	if !lwwWins(op.ClientUpdatedAt, row.UpdatedAt) {
		snap, err := wallSnapshot(row)
		if err != nil {
			return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
		}
		return pushStepResult{conflict: &synctypes.PushConflict{Reason: "stale", ServerVersion: snap}}
	}
	out, err := q.SoftDeleteWall(ctx, db.SoftDeleteWallParams{
		ID:        op.EntityID,
		UpdatedAt: epochMsToTimestamptz(op.ClientUpdatedAt),
	})
	if err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
	}
	return pushStepResult{applied: true, cursor: out.SyncCursor}
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
		return "", fmt.Errorf("parentType")
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
	if err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "validation", Message: err.Error()}}
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
		ownerType := string(payload.ParentType)
		pa, err := q.GetPhotoableByOwner(ctx, db.GetPhotoableByOwnerParams{
			OwnerType: ownerType,
			OwnerID:   payload.ParentID,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			pa, err = q.CreatePhotoable(ctx, db.CreatePhotoableParams{
				ID:        uuid.New().String(),
				OwnerType: ownerType,
				OwnerID:   payload.ParentID,
			})
			if err != nil {
				return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
			}
		} else if err != nil {
			return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
		}
		out, err := q.UpsertPhoto(ctx, db.UpsertPhotoParams{
			ID:          op.EntityID,
			PhotoableID: pa.ID,
			Name:        payload.Name,
			Caption:     payload.Caption,
			TakenAt:     takenAt,
			UpdatedAt:   epochMsToTimestamptz(op.ClientUpdatedAt),
		})
		if err != nil {
			return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
		}
		return pushStepResult{applied: true, cursor: out.SyncCursor}
	}
	if err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
	}

	phWS, err := workspaceOfPhoto(ctx, q, row.ID)
	if err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
	}
	if ve := validateWorkspace(phWS, wsID); ve != nil {
		return pushStepResult{pushError: ve}
	}

	if !lwwWins(op.ClientUpdatedAt, row.UpdatedAt) {
		snap, err := photoSnapshot(ctx, q, row)
		if err != nil {
			return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
		}
		return pushStepResult{conflict: &synctypes.PushConflict{Reason: "stale", ServerVersion: snap}}
	}

	out, err := q.UpsertPhoto(ctx, db.UpsertPhotoParams{
		ID:          op.EntityID,
		PhotoableID: row.PhotoableID,
		Name:        payload.Name,
		Caption:     payload.Caption,
		TakenAt:     takenAt,
		UpdatedAt:   epochMsToTimestamptz(op.ClientUpdatedAt),
	})
	if err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
	}
	return pushStepResult{applied: true, cursor: out.SyncCursor}
}

func (h *handler) pushPhotoDelete(ctx context.Context, q *db.Queries, wsID string, op synctypes.PushOperation) pushStepResult {
	row, err := q.GetPhotoByID(ctx, op.EntityID)
	if errors.Is(err, pgx.ErrNoRows) {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "notFound", Message: "photo not found"}}
	}
	if err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
	}
	phWS, err := workspaceOfPhoto(ctx, q, row.ID)
	if err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
	}
	if ve := validateWorkspace(phWS, wsID); ve != nil {
		return pushStepResult{pushError: ve}
	}
	if !lwwWins(op.ClientUpdatedAt, row.UpdatedAt) {
		snap, err := photoSnapshot(ctx, q, row)
		if err != nil {
			return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
		}
		return pushStepResult{conflict: &synctypes.PushConflict{Reason: "stale", ServerVersion: snap}}
	}
	out, err := q.SoftDeletePhoto(ctx, db.SoftDeletePhotoParams{
		ID:        op.EntityID,
		UpdatedAt: epochMsToTimestamptz(op.ClientUpdatedAt),
	})
	if err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
	}
	return pushStepResult{applied: true, cursor: out.SyncCursor}
}
