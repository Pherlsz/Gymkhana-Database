package taskengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5"
)

func (store *PostgresStore) CompleteJob(ctx context.Context, input CompleteJobInput) (Job, error) {
	if store == nil || store.pool == nil || input.JobID.IsZero() || input.Now.IsZero() || !input.State.Terminal() ||
		(input.State == JobCompleted && input.ErrorCode != "") || (input.State != JobCompleted && input.ErrorCode == "") ||
		len(input.Result.Compositions) > MaximumSolutions {
		return Job{}, ErrInvalidInput
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Job{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	current, sequence, err := lockTaskJob(ctx, tx, input.JobID)
	if err != nil {
		return Job{}, err
	}
	if current.State.Terminal() {
		if err := tx.Commit(ctx); err != nil {
			return Job{}, err
		}
		return current, nil
	}
	if current.State != JobRunning && current.State != JobQueued {
		return Job{}, ErrConflict
	}
	compositions := append([]Composition(nil), input.Result.Compositions...)
	sort.Slice(compositions, func(left, right int) bool { return compositions[left].Position < compositions[right].Position })
	for position := range compositions {
		compositions[position].Position = position
		raw, err := taskJSON(compositions[position], 5<<20)
		if err != nil || len(compositions[position].Evidence) > MaximumEvidencePerResult {
			return Job{}, ErrUnsafeResult
		}
		if _, err := tx.Exec(ctx, `INSERT INTO task_job_results(job_id,position,composition_json,evidence_count,created_at)
VALUES($1,$2,$3,$4,$5)`, taskUUID(input.JobID), position, raw, len(compositions[position].Evidence), input.Now); err != nil {
			return Job{}, taskPostgresError("store task composition", err)
		}
		if err := storeCandidateReferences(ctx, tx, input.JobID, compositions[position], input.Now); err != nil {
			return Job{}, err
		}
	}
	resultCount := len(compositions)
	kind := terminalEventKind(input.State)
	completed, err := scanJob(tx.QueryRow(ctx, `UPDATE task_jobs SET
state=$2,composition_count=$3,error_code=NULLIF($4,''),completed_at=$5,
next_event_sequence=next_event_sequence+1,version=version+1,updated_at=$5
WHERE id=$1 AND state IN ('QUEUED','RUNNING')
RETURNING `+jobColumns, taskUUID(input.JobID), string(input.State), resultCount, input.ErrorCode, input.Now))
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrConflict
	}
	if err != nil {
		return Job{}, taskPostgresError("complete task job", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO task_job_events
(job_id,sequence,event_kind,result_count,error_code,created_at)
VALUES($1,$2,$3,$4,NULLIF($5,''),$6)`, taskUUID(input.JobID), sequence, string(kind), resultCount, input.ErrorCode, input.Now); err != nil {
		return Job{}, taskPostgresError("record task terminal event", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Job{}, taskPostgresError("commit task completion", err)
	}
	return completed, nil
}

func storeCandidateReferences(ctx context.Context, tx pgx.Tx, jobID Identifier, composition Composition, now time.Time) error {
	roles := make([]string, 0, len(composition.Selected))
	for role := range composition.Selected {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	for _, role := range roles {
		for _, candidate := range composition.Selected[role] {
			if _, err := tx.Exec(ctx, `INSERT INTO task_candidate_references
(job_id,role_key,entity_kind,entity_id,entity_label,created_at)
VALUES($1,$2,$3,$4,$5,$6)
ON CONFLICT(job_id,role_key,entity_kind,entity_id) DO NOTHING`, taskUUID(jobID), role,
				candidate.Entity, candidate.ID, candidate.Label, now); err != nil {
				return taskPostgresError("store task candidate reference", err)
			}
		}
	}
	return nil
}

func (store *PostgresStore) FailJob(ctx context.Context, id Identifier, code string, state JobState, now time.Time) (Job, error) {
	if store == nil || store.pool == nil || id.IsZero() || code == "" || len(code) > 80 || now.IsZero() ||
		(state != JobFailed && state != JobCancelled) {
		return Job{}, ErrInvalidInput
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Job{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	current, _, err := lockTaskJob(ctx, tx, id)
	if err != nil {
		return Job{}, err
	}
	if current.State.Terminal() {
		if err := tx.Commit(ctx); err != nil {
			return Job{}, err
		}
		return current, nil
	}
	value, err := terminalTaskJobTx(ctx, tx, current, state, code, now)
	if err != nil {
		return Job{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Job{}, err
	}
	return value, nil
}

func terminalTaskJobTx(ctx context.Context, tx pgx.Tx, current Job, state JobState, code string, now time.Time) (Job, error) {
	var sequence int64
	if err := tx.QueryRow(ctx, `SELECT next_event_sequence FROM task_jobs WHERE id=$1`, taskUUID(current.ID)).Scan(&sequence); err != nil {
		return Job{}, err
	}
	value, err := scanJob(tx.QueryRow(ctx, `UPDATE task_jobs SET
state=$2,error_code=$3,completed_at=$4,next_event_sequence=next_event_sequence+1,
version=version+1,updated_at=$4 WHERE id=$1 AND state IN ('QUEUED','RUNNING')
RETURNING `+jobColumns, taskUUID(current.ID), string(state), code, now))
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrConflict
	}
	if err != nil {
		return Job{}, taskPostgresError("terminalize task job", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO task_job_events(job_id,sequence,event_kind,error_code,created_at)
VALUES($1,$2,$3,$4,$5)`, taskUUID(current.ID), sequence, string(terminalEventKind(state)), code, now); err != nil {
		return Job{}, taskPostgresError("record task failure event", err)
	}
	return value, nil
}

func terminalEventKind(state JobState) EventKind {
	switch state {
	case JobCompleted:
		return EventJobCompleted
	case JobIncomplete:
		return EventJobIncomplete
	case JobCancelled:
		return EventJobCancelled
	default:
		return EventJobFailed
	}
}

func lockTaskJob(ctx context.Context, tx pgx.Tx, id Identifier) (Job, int64, error) {
	current, err := scanJob(tx.QueryRow(ctx, `SELECT `+jobColumns+` FROM task_jobs WHERE id=$1 FOR UPDATE`, taskUUID(id)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, 0, ErrNotFound
	}
	if err != nil {
		return Job{}, 0, err
	}
	var sequence int64
	if err := tx.QueryRow(ctx, `SELECT next_event_sequence FROM task_jobs WHERE id=$1`, taskUUID(id)).Scan(&sequence); err != nil {
		return Job{}, 0, err
	}
	return current, sequence, nil
}

func (store *PostgresStore) RequestCancellation(ctx context.Context, id Identifier, owner auth.Identifier, now time.Time) (Job, error) {
	if store == nil || store.pool == nil || id.IsZero() || owner == (auth.Identifier{}) || now.IsZero() {
		return Job{}, ErrInvalidInput
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Job{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	current, _, err := lockTaskJob(ctx, tx, id)
	if err != nil {
		return Job{}, err
	}
	if current.OwnerUserID != owner {
		return Job{}, ErrNotFound
	}
	if current.State.Terminal() {
		return Job{}, ErrConflict
	}
	if current.State == JobQueued {
		value, err := terminalTaskJobTx(ctx, tx, current, JobCancelled, "cancelled", now)
		if err != nil {
			return Job{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return Job{}, err
		}
		return value, nil
	}
	value, err := scanJob(tx.QueryRow(ctx, `UPDATE task_jobs SET cancel_requested_at=COALESCE(cancel_requested_at,$3),
version=version+1,updated_at=$3 WHERE id=$1 AND owner_user_id=$2 AND state='RUNNING'
RETURNING `+jobColumns, taskUUID(id), taskAuthUUID(owner), now))
	if err != nil {
		return Job{}, taskPostgresError("request task cancellation", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Job{}, err
	}
	return value, nil
}

func (store *PostgresStore) ListEvents(ctx context.Context, id Identifier, owner auth.Identifier, after int64, limit int) (EventPage, error) {
	if store == nil || store.pool == nil || id.IsZero() || owner == (auth.Identifier{}) || after < 0 || limit < 1 || limit > MaximumEventPage {
		return EventPage{}, ErrInvalidInput
	}
	job, err := store.GetJob(ctx, id, owner)
	if err != nil {
		return EventPage{}, err
	}
	rows, err := store.pool.Query(ctx, `SELECT sequence,event_kind,progress_current,progress_total,candidate_count,result_count,error_code,created_at
FROM task_job_events WHERE job_id=$1 AND sequence>$2 ORDER BY sequence LIMIT $3`, taskUUID(id), after, limit)
	if err != nil {
		return EventPage{}, fmt.Errorf("list task events: %w", err)
	}
	defer rows.Close()
	page := EventPage{Events: make([]JobEvent, 0), LastSequence: after, Terminal: job.State.Terminal()}
	for rows.Next() {
		var event JobEvent
		var kind string
		var current, total, candidates, results *int
		var errorCode *string
		if err := rows.Scan(&event.Sequence, &kind, &current, &total, &candidates, &results, &errorCode, &event.CreatedAt); err != nil {
			return EventPage{}, fmt.Errorf("scan task event: %w", err)
		}
		event.JobID, event.Kind = id, EventKind(kind)
		event.ProgressCurrent, event.ProgressTotal = current, total
		event.CandidateCount, event.ResultCount = candidates, results
		if errorCode != nil {
			event.ErrorCode = *errorCode
		}
		page.Events = append(page.Events, event)
		page.LastSequence = event.Sequence
	}
	if err := rows.Err(); err != nil {
		return EventPage{}, fmt.Errorf("iterate task events: %w", err)
	}
	return page, nil
}

func (store *PostgresStore) ListResults(ctx context.Context, id Identifier, owner auth.Identifier, limit, offset int) (ResultPage, error) {
	job, err := store.GetJob(ctx, id, owner)
	if err != nil {
		return ResultPage{}, err
	}
	rows, err := store.pool.Query(ctx, `SELECT position,composition_json FROM task_job_results
WHERE job_id=$1 AND position>=$2 ORDER BY position LIMIT $3`, taskUUID(id), offset, limit)
	if err != nil {
		return ResultPage{}, fmt.Errorf("list task results: %w", err)
	}
	defer rows.Close()
	page := ResultPage{Job: job, Compositions: make([]Composition, 0), Total: job.CompositionCount, Limit: limit, Offset: offset}
	for rows.Next() {
		var position int
		var raw []byte
		if err := rows.Scan(&position, &raw); err != nil {
			return ResultPage{}, fmt.Errorf("scan task result: %w", err)
		}
		var composition Composition
		if err := json.Unmarshal(raw, &composition); err != nil || composition.Position != position {
			return ResultPage{}, ErrUnsafeResult
		}
		page.Compositions = append(page.Compositions, composition)
	}
	if err := rows.Err(); err != nil {
		return ResultPage{}, fmt.Errorf("iterate task results: %w", err)
	}
	return page, nil
}
