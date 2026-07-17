package googleforms

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (store *PostgresStore) CreateSource(ctx context.Context, value Source) (Source, error) {
	if store == nil || store.pool == nil || value.ID.IsZero() || value.ConnectionID.IsZero() ||
		value.OwnerUserID == (auth.Identifier{}) || !validProviderFormID(value.ProviderFormID) ||
		!value.Module.Valid() || value.SchemaFingerprint == ([32]byte{}) ||
		len(value.Questions) == 0 || len(value.Questions) > MaximumQuestions {
		return Source{}, ErrInvalidInput
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return Source{}, fmt.Errorf("begin source creation: %w", err)
	}
	defer tx.Rollback(ctx)
	ownerID := authDatabaseUUID(value.OwnerUserID)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 9109))`, ownerID); err != nil {
		return Source{}, fmt.Errorf("lock source owner: %w", err)
	}
	var connectionActive bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (
  SELECT 1 FROM google_forms_connections
   WHERE id=$1 AND owner_user_id=$2 AND state='ACTIVE'
)`, databaseUUID(value.ConnectionID), ownerID).Scan(&connectionActive); err != nil {
		return Source{}, fmt.Errorf("check source connection: %w", err)
	}
	if !connectionActive {
		return Source{}, ErrNeedsReauth
	}
	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM google_forms_sources WHERE owner_user_id=$1`, ownerID).Scan(&count); err != nil {
		return Source{}, fmt.Errorf("count owner sources: %w", err)
	}
	if count >= MaximumSourcesPerOwner {
		return Source{}, ErrConflict
	}
	created, err := scanSource(tx.QueryRow(ctx, `INSERT INTO google_forms_sources (
  id, connection_id, owner_user_id, provider_form_id, title, module, state,
  schema_revision, schema_fingerprint, sync_mode, poll_interval_seconds,
  created_at, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,'DRAFT',$7,$8,'MANUAL',$9,$10,$10)
RETURNING id, connection_id, owner_user_id, provider_form_id, title,
          module, state, schema_revision, schema_fingerprint, sync_mode,
	          poll_interval_seconds, cursor_submitted_at, response_page_token,
	          page_token_cursor_started_at, last_synced_at, next_sync_at,
          error_code, version, created_at, updated_at`, databaseUUID(value.ID),
		databaseUUID(value.ConnectionID), ownerID, value.ProviderFormID, value.Title,
		value.Module, value.SchemaRevision, value.SchemaFingerprint[:],
		int(value.PollInterval/time.Second), value.CreatedAt))
	if err != nil {
		return Source{}, mapPostgresError("create source", err)
	}
	if err := insertQuestions(ctx, tx, created.ID, value.Questions, nil); err != nil {
		return Source{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Source{}, fmt.Errorf("commit source creation: %w", err)
	}
	created.Questions = append([]Question(nil), value.Questions...)
	return created, nil
}

func (store *PostgresStore) GetSource(ctx context.Context, id Identifier, ownerID auth.Identifier) (Source, error) {
	value, err := scanSource(store.pool.QueryRow(ctx, sourceSelect+` WHERE id=$1 AND owner_user_id=$2`, databaseUUID(id), authDatabaseUUID(ownerID)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Source{}, ErrNotFound
	}
	if err != nil {
		return Source{}, fmt.Errorf("get source: %w", err)
	}
	value.Questions, err = store.loadQuestions(ctx, id)
	if err != nil {
		return Source{}, err
	}
	return value, nil
}

func (store *PostgresStore) ListSources(ctx context.Context, ownerID auth.Identifier, limit, offset int) (SourcePage, error) {
	if limit < 1 || limit > 100 || offset < 0 {
		return SourcePage{}, ErrInvalidInput
	}
	var total int
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM google_forms_sources WHERE owner_user_id=$1`, authDatabaseUUID(ownerID)).Scan(&total); err != nil {
		return SourcePage{}, fmt.Errorf("count sources: %w", err)
	}
	rows, err := store.pool.Query(ctx, sourceSelect+` WHERE owner_user_id=$1 ORDER BY created_at DESC, id DESC LIMIT $2 OFFSET $3`, authDatabaseUUID(ownerID), limit, offset)
	if err != nil {
		return SourcePage{}, fmt.Errorf("list sources: %w", err)
	}
	defer rows.Close()
	page := SourcePage{Total: total}
	for rows.Next() {
		value, scanErr := scanSource(rows)
		if scanErr != nil {
			return SourcePage{}, fmt.Errorf("scan source: %w", scanErr)
		}
		page.Sources = append(page.Sources, value)
	}
	if err := rows.Err(); err != nil {
		return SourcePage{}, fmt.Errorf("iterate sources: %w", err)
	}
	for index := range page.Sources {
		page.Sources[index].Questions, err = store.loadQuestions(ctx, page.Sources[index].ID)
		if err != nil {
			return SourcePage{}, err
		}
	}
	return page, nil
}

func (store *PostgresStore) loadQuestions(ctx context.Context, sourceID Identifier) ([]Question, error) {
	rows, err := store.pool.Query(ctx, `SELECT question_id, position, title, answer_kind,
       required, supported, unsupported_code, question_fingerprint, target_field
  FROM google_forms_questions
 WHERE source_id=$1
 ORDER BY position`, databaseUUID(sourceID))
	if err != nil {
		return nil, fmt.Errorf("list source questions: %w", err)
	}
	defer rows.Close()
	result := make([]Question, 0)
	for rows.Next() {
		var value Question
		var unsupported, target pgtype.Text
		var fingerprint []byte
		if err := rows.Scan(&value.ID, &value.Position, &value.Title, &value.AnswerKind,
			&value.Required, &value.Supported, &unsupported, &fingerprint, &target); err != nil {
			return nil, fmt.Errorf("scan source question: %w", err)
		}
		copy(value.QuestionFingerprint[:], fingerprint)
		if unsupported.Valid {
			value.UnsupportedCode = unsupported.String
		}
		if target.Valid {
			value.TargetField = target.String
		}
		result = append(result, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate source questions: %w", err)
	}
	return result, nil
}

func insertQuestions(ctx context.Context, tx pgx.Tx, sourceID Identifier, questions []Question, targets map[string]string) error {
	for _, question := range questions {
		target := question.TargetField
		if targets != nil {
			target = targets[question.ID]
		}
		if _, err := tx.Exec(ctx, `INSERT INTO google_forms_questions (
  source_id, question_id, position, title, answer_kind, required, supported,
  unsupported_code, question_fingerprint, target_field
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, databaseUUID(sourceID), question.ID,
			question.Position, question.Title, question.AnswerKind, question.Required,
			question.Supported, optionalText(question.UnsupportedCode),
			question.QuestionFingerprint[:], optionalText(target)); err != nil {
			return mapPostgresError("insert source question", err)
		}
	}
	return nil
}

func (store *PostgresStore) SaveMapping(ctx context.Context, id Identifier, ownerID auth.Identifier, version int64, mapping []MappingInput, now time.Time) (Source, error) {
	if len(mapping) == 0 || len(mapping) > MaximumQuestions {
		return Source{}, ErrInvalidInput
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return Source{}, fmt.Errorf("begin source mapping: %w", err)
	}
	defer tx.Rollback(ctx)
	var state SourceState
	var currentVersion int64
	if err := tx.QueryRow(ctx, `SELECT state, version FROM google_forms_sources
 WHERE id=$1 AND owner_user_id=$2 FOR UPDATE`, databaseUUID(id), authDatabaseUUID(ownerID)).Scan(&state, &currentVersion); errors.Is(err, pgx.ErrNoRows) {
		return Source{}, ErrNotFound
	} else if err != nil {
		return Source{}, fmt.Errorf("lock source mapping: %w", err)
	}
	if currentVersion != version {
		return Source{}, ErrConflict
	}
	if state == SourceNeedsReauth || state == SourceError {
		return Source{}, ErrInvalidState
	}
	if _, err := tx.Exec(ctx, `UPDATE google_forms_questions SET target_field=NULL WHERE source_id=$1`, databaseUUID(id)); err != nil {
		return Source{}, fmt.Errorf("clear source mapping: %w", err)
	}
	seenQuestions := make(map[string]struct{}, len(mapping))
	seenTargets := make(map[string]struct{}, len(mapping))
	for _, item := range mapping {
		if _, duplicate := seenQuestions[item.QuestionID]; duplicate {
			return Source{}, ErrInvalidInput
		}
		if _, duplicate := seenTargets[item.TargetField]; duplicate {
			return Source{}, ErrInvalidInput
		}
		seenQuestions[item.QuestionID] = struct{}{}
		seenTargets[item.TargetField] = struct{}{}
		command, err := tx.Exec(ctx, `UPDATE google_forms_questions SET target_field=$3
 WHERE source_id=$1 AND question_id=$2 AND supported=true`, databaseUUID(id), item.QuestionID, item.TargetField)
		if err != nil {
			return Source{}, mapPostgresError("save source mapping item", err)
		}
		if command.RowsAffected() != 1 {
			return Source{}, ErrInvalidInput
		}
	}
	nextState := SourcePaused
	if state == SourceActive {
		nextState = SourceActive
	}
	if _, err := tx.Exec(ctx, `UPDATE google_forms_sources
   SET state=$3, error_code=NULL,
       next_sync_at=CASE WHEN $3='ACTIVE' AND sync_mode='POLL' THEN $4::timestamptz+make_interval(secs=>poll_interval_seconds) ELSE NULL END,
       version=version+1, updated_at=$4
 WHERE id=$1 AND owner_user_id=$2`, databaseUUID(id), authDatabaseUUID(ownerID), nextState, now); err != nil {
		return Source{}, fmt.Errorf("finish source mapping: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Source{}, fmt.Errorf("commit source mapping: %w", err)
	}
	return store.GetSource(ctx, id, ownerID)
}

func (store *PostgresStore) UpdateSource(ctx context.Context, id Identifier, ownerID auth.Identifier, version int64, input UpdateSourceInput, now time.Time) (Source, error) {
	if input.SyncMode != SyncManual && input.SyncMode != SyncPoll || input.PollInterval < 5*time.Minute || input.PollInterval > 24*time.Hour {
		return Source{}, ErrInvalidInput
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return Source{}, fmt.Errorf("begin source update: %w", err)
	}
	defer tx.Rollback(ctx)
	var state SourceState
	var currentVersion int64
	var mapped int
	if err := tx.QueryRow(ctx, `SELECT state, version,
       (SELECT count(*) FROM google_forms_questions q WHERE q.source_id=s.id AND q.target_field IS NOT NULL)
  FROM google_forms_sources s
 WHERE id=$1 AND owner_user_id=$2 FOR UPDATE`, databaseUUID(id), authDatabaseUUID(ownerID)).Scan(&state, &currentVersion, &mapped); errors.Is(err, pgx.ErrNoRows) {
		return Source{}, ErrNotFound
	} else if err != nil {
		return Source{}, fmt.Errorf("lock source update: %w", err)
	}
	if currentVersion != version {
		return Source{}, ErrConflict
	}
	if state == SourceNeedsReauth || state == SourceSchemaDrift || state == SourceError {
		return Source{}, ErrInvalidState
	}
	if input.Enabled && mapped == 0 {
		return Source{}, ErrInvalidState
	}
	nextState := SourcePaused
	if input.Enabled {
		nextState = SourceActive
	}
	seconds := int(input.PollInterval / time.Second)
	if _, err := tx.Exec(ctx, `UPDATE google_forms_sources
   SET state=$4, sync_mode=$5, poll_interval_seconds=$6,
       next_sync_at=CASE WHEN $4='ACTIVE' AND $5='POLL' THEN $7::timestamptz+make_interval(secs=>$6::integer) ELSE NULL END,
       error_code=NULL, version=version+1, updated_at=$7
 WHERE id=$1 AND owner_user_id=$2 AND version=$3`, databaseUUID(id), authDatabaseUUID(ownerID), version,
		nextState, input.SyncMode, seconds, now); err != nil {
		return Source{}, fmt.Errorf("update source: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Source{}, fmt.Errorf("commit source update: %w", err)
	}
	return store.GetSource(ctx, id, ownerID)
}

func (store *PostgresStore) RefreshSourceSchema(ctx context.Context, id Identifier, ownerID auth.Identifier, form Form, now time.Time) (Source, bool, error) {
	if form.Fingerprint == ([32]byte{}) || len(form.Questions) == 0 || len(form.Questions) > MaximumQuestions {
		return Source{}, false, ErrInvalidInput
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return Source{}, false, fmt.Errorf("begin source schema refresh: %w", err)
	}
	defer tx.Rollback(ctx)
	var currentFingerprint []byte
	var state SourceState
	if err := tx.QueryRow(ctx, `SELECT schema_fingerprint, state FROM google_forms_sources
 WHERE id=$1 AND owner_user_id=$2 FOR UPDATE`, databaseUUID(id), authDatabaseUUID(ownerID)).Scan(&currentFingerprint, &state); errors.Is(err, pgx.ErrNoRows) {
		return Source{}, false, ErrNotFound
	} else if err != nil {
		return Source{}, false, fmt.Errorf("lock source schema: %w", err)
	}
	var current [32]byte
	copy(current[:], currentFingerprint)
	drifted := current != form.Fingerprint
	targets := make(map[string]string)
	rows, err := tx.Query(ctx, `SELECT question_id, target_field, question_fingerprint
  FROM google_forms_questions WHERE source_id=$1`, databaseUUID(id))
	if err != nil {
		return Source{}, false, fmt.Errorf("load source mapping for schema refresh: %w", err)
	}
	oldFingerprints := make(map[string][32]byte)
	for rows.Next() {
		var questionID string
		var target pgtype.Text
		var fingerprint []byte
		if err := rows.Scan(&questionID, &target, &fingerprint); err != nil {
			rows.Close()
			return Source{}, false, fmt.Errorf("scan schema mapping: %w", err)
		}
		var digest [32]byte
		copy(digest[:], fingerprint)
		oldFingerprints[questionID] = digest
		if target.Valid {
			targets[questionID] = target.String
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Source{}, false, fmt.Errorf("iterate schema mapping: %w", err)
	}
	rows.Close()
	if drifted {
		for _, question := range form.Questions {
			if old, exists := oldFingerprints[question.ID]; !exists || old != question.QuestionFingerprint {
				delete(targets, question.ID)
			}
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM google_forms_questions WHERE source_id=$1`, databaseUUID(id)); err != nil {
		return Source{}, false, fmt.Errorf("replace source questions: %w", err)
	}
	if err := insertQuestions(ctx, tx, id, form.Questions, targets); err != nil {
		return Source{}, false, err
	}
	nextState := state
	if drifted {
		nextState = SourceSchemaDrift
	}
	if _, err := tx.Exec(ctx, `UPDATE google_forms_sources
	   SET title=$3, schema_revision=$4, schema_fingerprint=$5, state=$6,
	       next_sync_at=CASE WHEN $6='SCHEMA_DRIFT' THEN NULL ELSE next_sync_at END,
	       cursor_submitted_at=CASE WHEN $6='SCHEMA_DRIFT'
	         THEN COALESCE(page_token_cursor_started_at, cursor_submitted_at)
	         ELSE cursor_submitted_at END,
	       response_page_token=CASE WHEN $6='SCHEMA_DRIFT' THEN NULL ELSE response_page_token END,
	       page_token_cursor_started_at=CASE WHEN $6='SCHEMA_DRIFT' THEN NULL ELSE page_token_cursor_started_at END,
       error_code=CASE WHEN $6='SCHEMA_DRIFT' THEN 'schema_drift' ELSE error_code END,
       version=version+1, updated_at=$7
 WHERE id=$1 AND owner_user_id=$2`, databaseUUID(id), authDatabaseUUID(ownerID), form.Title,
		form.Revision, form.Fingerprint[:], nextState, now); err != nil {
		return Source{}, false, fmt.Errorf("update source schema: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Source{}, false, fmt.Errorf("commit source schema refresh: %w", err)
	}
	value, err := store.GetSource(ctx, id, ownerID)
	return value, drifted, err
}

func (store *PostgresStore) DueSources(ctx context.Context, now time.Time, limit int) ([]Source, error) {
	if limit < 1 || limit > 100 {
		return nil, ErrInvalidInput
	}
	rows, err := store.pool.Query(ctx, sourceSelect+` WHERE state='ACTIVE'
	   AND (response_page_token IS NOT NULL OR
	        (sync_mode='POLL' AND next_sync_at IS NOT NULL AND next_sync_at<=$1))
   AND NOT EXISTS (
     SELECT 1 FROM google_forms_sync_runs run
      WHERE run.source_id=google_forms_sources.id AND run.state IN ('QUEUED','RUNNING')
   )
	 ORDER BY response_page_token IS NULL, next_sync_at NULLS LAST, id LIMIT $2`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("list due sources: %w", err)
	}
	defer rows.Close()
	result := make([]Source, 0, limit)
	for rows.Next() {
		value, scanErr := scanSource(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan due source: %w", scanErr)
		}
		result = append(result, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate due sources: %w", err)
	}
	return result, nil
}
