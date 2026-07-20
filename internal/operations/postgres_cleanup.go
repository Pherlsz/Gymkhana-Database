package operations

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func (store *PostgresStore) ClaimCleanupCandidates(ctx context.Context, now time.Time, limit int) ([]CleanupCandidate, error) {
	if limit < 1 || limit > 1000 {
		return nil, ErrInvalidInput
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin operation cleanup claim: %w", err)
	}
	defer tx.Rollback(ctx)
	result := make([]CleanupCandidate, 0, limit)
	rows, err := tx.Query(ctx, `WITH candidates AS (
  SELECT id FROM operation_imports
   WHERE expires_at<=$1 AND object_deleted_at IS NULL
     AND (state NOT IN ('PARSING','RUNNING') OR updated_at<=$3)
     AND (cleanup_claimed_at IS NULL OR cleanup_claimed_at<=$3)
   ORDER BY expires_at, id
   FOR UPDATE SKIP LOCKED
   LIMIT $2
)
UPDATE operation_imports value
   SET state='EXPIRED', stage='CLEANUP', cleanup_claimed_at=$1,
       version=CASE WHEN value.state='EXPIRED' THEN value.version ELSE value.version+1 END,
       updated_at=$1
  FROM candidates
 WHERE value.id=candidates.id
RETURNING value.id, value.actor_user_id, value.module, value.object_key`, now, limit, now.Add(-CleanupClaimTTL))
	if err != nil {
		return nil, fmt.Errorf("claim expired imports: %w", err)
	}
	for rows.Next() {
		value := CleanupCandidate{Kind: "IMPORT"}
		var id, actorID pgtype.UUID
		var objectKey pgtype.Text
		if err := rows.Scan(&id, &actorID, &value.Module, &objectKey); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan expired import: %w", err)
		}
		value.ID = identifierFromUUID(id)
		value.ActorUserID = authIdentifierFromUUID(actorID)
		if objectKey.Valid {
			value.ObjectKey = objectKey.String
			value.RequiresObjectDeletion = true
		}
		result = append(result, value)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate expired imports: %w", err)
	}
	rows.Close()
	remaining := limit - len(result)
	if remaining > 0 {
		rows, err = tx.Query(ctx, `WITH candidates AS (
  SELECT id FROM operation_exports
   WHERE expires_at<=$1 AND object_deleted_at IS NULL
     AND (state<>'RUNNING' OR updated_at<=$3)
     AND (cleanup_claimed_at IS NULL OR cleanup_claimed_at<=$3)
   ORDER BY expires_at, id
   FOR UPDATE SKIP LOCKED
   LIMIT $2
)
UPDATE operation_exports value
   SET state='EXPIRED', cleanup_claimed_at=$1,
       version=CASE WHEN value.state='EXPIRED' THEN value.version ELSE value.version+1 END,
       updated_at=$1
  FROM candidates
 WHERE value.id=candidates.id
RETURNING value.id, value.actor_user_id, value.module, value.object_key`, now, remaining, now.Add(-CleanupClaimTTL))
		if err != nil {
			return nil, fmt.Errorf("claim expired exports: %w", err)
		}
		for rows.Next() {
			value := CleanupCandidate{Kind: "EXPORT"}
			var id, actorID pgtype.UUID
			if err := rows.Scan(&id, &actorID, &value.Module, &value.ObjectKey); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan expired export: %w", err)
			}
			value.ID = identifierFromUUID(id)
			value.ActorUserID = authIdentifierFromUUID(actorID)
			value.RequiresObjectDeletion = true
			result = append(result, value)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("iterate expired exports: %w", err)
		}
		rows.Close()
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit operation cleanup claim: %w", err)
	}
	return result, nil
}

func (store *PostgresStore) MarkObjectDeleted(ctx context.Context, candidate CleanupCandidate, now time.Time) error {
	var query string
	switch candidate.Kind {
	case "IMPORT":
		query = `UPDATE operation_imports SET object_deleted_at=$2, cleanup_claimed_at=NULL, updated_at=$2 WHERE id=$1 AND state='EXPIRED' AND object_deleted_at IS NULL`
	case "EXPORT":
		query = `UPDATE operation_exports SET object_deleted_at=$2, cleanup_claimed_at=NULL, updated_at=$2 WHERE id=$1 AND state='EXPIRED' AND object_deleted_at IS NULL`
	default:
		return ErrInvalidInput
	}
	if _, err := store.pool.Exec(ctx, query, databaseUUID(candidate.ID), now); err != nil {
		return fmt.Errorf("mark operation object deleted: %w", err)
	}
	return nil
}

func (store *PostgresStore) ReleaseCleanupCandidate(ctx context.Context, candidate CleanupCandidate) error {
	var query string
	switch candidate.Kind {
	case "IMPORT":
		query = `UPDATE operation_imports SET cleanup_claimed_at=NULL WHERE id=$1 AND object_deleted_at IS NULL`
	case "EXPORT":
		query = `UPDATE operation_exports SET cleanup_claimed_at=NULL WHERE id=$1 AND object_deleted_at IS NULL`
	default:
		return ErrInvalidInput
	}
	if _, err := store.pool.Exec(ctx, query, databaseUUID(candidate.ID)); err != nil {
		return fmt.Errorf("release operation cleanup claim: %w", err)
	}
	return nil
}
