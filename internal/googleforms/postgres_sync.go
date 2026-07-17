package googleforms

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/operations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (store *PostgresStore) CreateSyncRun(ctx context.Context, value SyncRun) (SyncRun, error) {
	if store == nil || store.pool == nil || value.ID.IsZero() || value.SourceID.IsZero() ||
		value.OwnerUserID == (auth.Identifier{}) ||
		(value.TriggerKind != TriggerManual && value.TriggerKind != TriggerScheduled) ||
		len(value.IdempotencyKey) < 8 || len(value.IdempotencyKey) > 128 {
		return SyncRun{}, ErrInvalidInput
	}
	created, err := scanSyncRun(store.pool.QueryRow(ctx, `INSERT INTO google_forms_sync_runs (
  id, source_id, owner_user_id, actor_user_id, trigger_kind, state,
  idempotency_key, cursor_started_at, created_at, updated_at
) VALUES ($1,$2,$3,$4,$5,'QUEUED',$6,$7,$8,$8)
ON CONFLICT (source_id, idempotency_key) DO UPDATE
  SET idempotency_key=EXCLUDED.idempotency_key
RETURNING id, source_id, owner_user_id, actor_user_id, trigger_kind,
          state, idempotency_key, operation_import_id, cursor_started_at,
          cursor_completed_at, received_count, staged_count, duplicate_count,
          river_job_id, error_code, started_at, completed_at, version, created_at, updated_at`,
		databaseUUID(value.ID), databaseUUID(value.SourceID), authDatabaseUUID(value.OwnerUserID),
		optionalAuthDatabaseUUID(value.ActorUserID), value.TriggerKind, value.IdempotencyKey,
		optionalTime(value.CursorStartedAt), value.CreatedAt))
	if err != nil {
		return SyncRun{}, mapPostgresError("create sync run", err)
	}
	return created, nil
}

func (store *PostgresStore) SetSyncJob(ctx context.Context, id Identifier, jobID int64, now time.Time) error {
	if jobID <= 0 {
		return ErrInvalidInput
	}
	command, err := store.pool.Exec(ctx, `UPDATE google_forms_sync_runs
   SET river_job_id=$2, version=version+1, updated_at=$3
 WHERE id=$1 AND state='QUEUED' AND (river_job_id IS NULL OR river_job_id=$2)`, databaseUUID(id), jobID, now)
	if err != nil {
		return fmt.Errorf("set sync job: %w", err)
	}
	if command.RowsAffected() != 1 {
		return ErrConflict
	}
	return nil
}

func (store *PostgresStore) BeginSync(ctx context.Context, id Identifier, now time.Time) (SyncRun, Source, Connection, auth.Session, error) {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return SyncRun{}, Source{}, Connection{}, auth.Session{}, fmt.Errorf("begin sync: %w", err)
	}
	defer tx.Rollback(ctx)
	run, err := scanSyncRun(tx.QueryRow(ctx, syncRunSelect+` WHERE id=$1 FOR UPDATE`, databaseUUID(id)))
	if errors.Is(err, pgx.ErrNoRows) {
		return SyncRun{}, Source{}, Connection{}, auth.Session{}, ErrNotFound
	}
	if err != nil {
		return SyncRun{}, Source{}, Connection{}, auth.Session{}, fmt.Errorf("lock sync run: %w", err)
	}
	switch run.State {
	case SyncCancelled:
		return SyncRun{}, Source{}, Connection{}, auth.Session{}, ErrCancelled
	case SyncCompleted, SyncFailed:
		return SyncRun{}, Source{}, Connection{}, auth.Session{}, ErrInvalidState
	case SyncQueued:
		run, err = scanSyncRun(tx.QueryRow(ctx, `UPDATE google_forms_sync_runs
   SET state='RUNNING', started_at=COALESCE(started_at,$2), version=version+1, updated_at=$2
 WHERE id=$1
RETURNING id, source_id, owner_user_id, actor_user_id, trigger_kind,
          state, idempotency_key, operation_import_id, cursor_started_at,
          cursor_completed_at, received_count, staged_count, duplicate_count,
          river_job_id, error_code, started_at, completed_at, version, created_at, updated_at`, databaseUUID(id), now))
		if err != nil {
			return SyncRun{}, Source{}, Connection{}, auth.Session{}, fmt.Errorf("start sync run: %w", err)
		}
	case SyncRunning:
	default:
		return SyncRun{}, Source{}, Connection{}, auth.Session{}, ErrInvalidState
	}
	source, err := scanSource(tx.QueryRow(ctx, sourceSelect+` WHERE id=$1 AND owner_user_id=$2 FOR SHARE`, databaseUUID(run.SourceID), authDatabaseUUID(run.OwnerUserID)))
	if errors.Is(err, pgx.ErrNoRows) {
		return SyncRun{}, Source{}, Connection{}, auth.Session{}, ErrNotFound
	}
	if err != nil {
		return SyncRun{}, Source{}, Connection{}, auth.Session{}, fmt.Errorf("load sync source: %w", err)
	}
	if source.State != SourceActive {
		if source.State == SourceNeedsReauth {
			return SyncRun{}, Source{}, Connection{}, auth.Session{}, ErrNeedsReauth
		}
		if source.State == SourceSchemaDrift {
			return SyncRun{}, Source{}, Connection{}, auth.Session{}, ErrSchemaDrift
		}
		return SyncRun{}, Source{}, Connection{}, auth.Session{}, ErrInvalidState
	}
	connection, err := scanConnection(tx.QueryRow(ctx, connectionSelect+` WHERE id=$1 AND owner_user_id=$2 FOR SHARE`, databaseUUID(source.ConnectionID), authDatabaseUUID(source.OwnerUserID)))
	if errors.Is(err, pgx.ErrNoRows) {
		return SyncRun{}, Source{}, Connection{}, auth.Session{}, ErrNotFound
	}
	if err != nil {
		return SyncRun{}, Source{}, Connection{}, auth.Session{}, fmt.Errorf("load sync connection: %w", err)
	}
	if connection.State != ConnectionActive {
		return SyncRun{}, Source{}, Connection{}, auth.Session{}, ErrNeedsReauth
	}
	actorID := run.OwnerUserID
	if run.ActorUserID != nil {
		actorID = *run.ActorUserID
	}
	actor, err := scanActor(tx.QueryRow(ctx, `SELECT id, github_user_id, github_login, display_name,
       avatar_url, role, active FROM app_users WHERE id=$1`, authDatabaseUUID(actorID)))
	if errors.Is(err, pgx.ErrNoRows) {
		return SyncRun{}, Source{}, Connection{}, auth.Session{}, ErrForbidden
	}
	if err != nil {
		return SyncRun{}, Source{}, Connection{}, auth.Session{}, fmt.Errorf("load sync actor: %w", err)
	}
	if !actor.User.Active || !actor.User.Role.CanManageGoogleForms() || actor.User.ID != source.OwnerUserID {
		return SyncRun{}, Source{}, Connection{}, auth.Session{}, ErrForbidden
	}
	if err := tx.Commit(ctx); err != nil {
		return SyncRun{}, Source{}, Connection{}, auth.Session{}, fmt.Errorf("commit sync start: %w", err)
	}
	return run, source, connection, actor, nil
}

func scanActor(row rowScanner) (auth.Session, error) {
	var session auth.Session
	var id pgtype.UUID
	var avatar pgtype.Text
	if err := row.Scan(&id, &session.User.GitHubUserID, &session.User.Login,
		&session.User.DisplayName, &avatar, &session.User.Role, &session.User.Active); err != nil {
		return auth.Session{}, err
	}
	session.User.ID = authIdentifierFromUUID(id)
	if avatar.Valid {
		session.User.AvatarURL = avatar.String
	}
	return session, nil
}

func (store *PostgresStore) StageResponses(ctx context.Context, run SyncRun, source Source, responses []Response, now, expiresAt time.Time) (StageResult, error) {
	if run.ID.IsZero() || source.ID.IsZero() || run.SourceID != source.ID ||
		len(responses) > MaximumResponsesPerRun*MaximumPagesPerRun || !expiresAt.After(now) {
		return StageResult{}, ErrInvalidInput
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return StageResult{}, fmt.Errorf("begin response staging: %w", err)
	}
	defer tx.Rollback(ctx)
	locked, err := scanSyncRun(tx.QueryRow(ctx, syncRunSelect+` WHERE id=$1 FOR UPDATE`, databaseUUID(run.ID)))
	if errors.Is(err, pgx.ErrNoRows) {
		return StageResult{}, ErrNotFound
	}
	if err != nil {
		return StageResult{}, fmt.Errorf("lock response staging: %w", err)
	}
	if locked.State == SyncCancelled {
		return StageResult{}, ErrCancelled
	}
	if locked.State != SyncRunning {
		return StageResult{}, ErrInvalidState
	}
	if locked.OperationImportID != nil || locked.ReceivedCount > 0 {
		result := StageResult{ReceivedCount: locked.ReceivedCount, StagedCount: locked.StagedCount,
			DuplicateCount: locked.DuplicateCount, Cursor: locked.CursorCompletedAt}
		if locked.OperationImportID != nil {
			var version int64
			if err := tx.QueryRow(ctx, `SELECT version FROM operation_imports WHERE id=$1`, pgtype.UUID{Bytes: [16]byte(*locked.OperationImportID), Valid: true}).Scan(&version); err != nil {
				return StageResult{}, fmt.Errorf("load staged import: %w", err)
			}
			result.Import = &operations.Import{ID: *locked.OperationImportID, ActorUserID: locked.OwnerUserID,
				Module: source.Module, SourceKind: operations.SourceGoogleForms, Version: version}
		}
		if err := tx.Commit(ctx); err != nil {
			return StageResult{}, fmt.Errorf("commit replayed response staging: %w", err)
		}
		return result, nil
	}
	source.Questions, err = loadQuestionsTx(ctx, tx, source.ID)
	if err != nil {
		return StageResult{}, err
	}
	questions := make([]Question, 0, len(source.Questions))
	for _, question := range source.Questions {
		if question.Supported {
			questions = append(questions, question)
		}
	}
	if len(questions) == 0 {
		return StageResult{}, ErrUnsupportedForm
	}
	values := sortedResponses(responses)
	ids := make([]string, 0, len(values))
	seenInput := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value.ID == "" || len(value.ID) > 500 || value.SubmittedAt.IsZero() {
			return StageResult{}, ErrInvalidInput
		}
		if _, duplicate := seenInput[value.ID]; duplicate {
			continue
		}
		seenInput[value.ID] = struct{}{}
		ids = append(ids, value.ID)
	}
	existing := make(map[string]struct{})
	if len(ids) > 0 {
		rows, queryErr := tx.Query(ctx, `SELECT provider_response_id
  FROM google_forms_response_receipts
 WHERE source_id=$1 AND provider_response_id=ANY($2::text[])`, databaseUUID(source.ID), ids)
		if queryErr != nil {
			return StageResult{}, fmt.Errorf("load response receipts: %w", queryErr)
		}
		for rows.Next() {
			var responseID string
			if scanErr := rows.Scan(&responseID); scanErr != nil {
				rows.Close()
				return StageResult{}, fmt.Errorf("scan response receipt: %w", scanErr)
			}
			existing[responseID] = struct{}{}
		}
		if rowsErr := rows.Err(); rowsErr != nil {
			rows.Close()
			return StageResult{}, fmt.Errorf("iterate response receipts: %w", rowsErr)
		}
		rows.Close()
	}
	selected := make([]Response, 0, min(len(values), MaximumResponsesPerRun))
	processedIDs := make(map[string]struct{}, len(values))
	received, duplicates := 0, 0
	var cursor *time.Time
	for _, value := range values {
		if _, duplicate := processedIDs[value.ID]; duplicate {
			continue
		}
		processedIDs[value.ID] = struct{}{}
		if len(selected) >= MaximumResponsesPerRun {
			break
		}
		received++
		timestamp := value.SubmittedAt.UTC()
		cursor = &timestamp
		if _, duplicate := existing[value.ID]; duplicate {
			duplicates++
			continue
		}
		selected = append(selected, value)
	}
	result := StageResult{ReceivedCount: received, StagedCount: len(selected), DuplicateCount: duplicates, Cursor: cursor}
	if len(selected) == 0 {
		if _, err := tx.Exec(ctx, `UPDATE google_forms_sync_runs
   SET cursor_completed_at=$2, received_count=$3, staged_count=0,
       duplicate_count=$4, version=version+1, updated_at=$5
 WHERE id=$1 AND state='RUNNING'`, databaseUUID(run.ID), optionalTime(cursor), received, duplicates, now); err != nil {
			return StageResult{}, fmt.Errorf("save empty response staging: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return StageResult{}, fmt.Errorf("commit empty response staging: %w", err)
		}
		return result, nil
	}
	operationID := run.ID.OperationIdentifier()
	operationUUID := pgtype.UUID{Bytes: [16]byte(operationID), Valid: true}
	if _, err := tx.Exec(ctx, `INSERT INTO operation_imports (
  id, actor_user_id, module, source_kind, source_reference_id,
  idempotency_key, state, stage, selected_sheet_index, mapping_version,
  expires_at, created_at, updated_at
) VALUES ($1,$2,$3,'GOOGLE_FORMS',$4,$5,'MAPPING','MAP',0,1,$6,$7,$7)`, operationUUID,
		authDatabaseUUID(source.OwnerUserID), source.Module, databaseUUID(source.ID),
		"google-forms:"+run.ID.String(), expiresAt, now); err != nil {
		return StageResult{}, mapPostgresError("create response import", err)
	}
	sheetName := truncateText(source.Title, 120)
	if _, err := tx.Exec(ctx, `INSERT INTO operation_import_sheets (
  import_id, sheet_index, name, row_count, column_count
) VALUES ($1,0,$2,$3,$4)`, operationUUID, sheetName, len(selected), len(questions)); err != nil {
		return StageResult{}, mapPostgresError("create response sheet", err)
	}
	for _, question := range questions {
		if _, err := tx.Exec(ctx, `INSERT INTO operation_import_columns (
  import_id, sheet_index, source_column, source_header, target_field
) VALUES ($1,0,$2,$3,$4)`, operationUUID, question.Position, question.Title, optionalText(question.TargetField)); err != nil {
			return StageResult{}, mapPostgresError("create response column", err)
		}
	}
	for index, response := range selected {
		rowNumber := index + 2
		if _, err := tx.Exec(ctx, `INSERT INTO operation_import_rows (import_id, sheet_index, row_number)
VALUES ($1,0,$2)`, operationUUID, rowNumber); err != nil {
			return StageResult{}, mapPostgresError("create response row", err)
		}
		for _, question := range questions {
			raw, kind, validation := responseCell(response, question)
			if _, err := tx.Exec(ctx, `INSERT INTO operation_import_cells (
  import_id, sheet_index, row_number, source_column, raw_value, value_kind,
  validation_code, source_validation_code
) VALUES ($1,0,$2,$3,$4,$5,$6,$6)`, operationUUID, rowNumber, question.Position,
				raw, kind, optionalText(validation)); err != nil {
				return StageResult{}, mapPostgresError("create response cell", err)
			}
		}
		fingerprint := responseFingerprint(response)
		if _, err := tx.Exec(ctx, `INSERT INTO google_forms_response_receipts (
  source_id, provider_response_id, submitted_at, response_fingerprint,
  import_id, sheet_index, row_number, created_at
) VALUES ($1,$2,$3,$4,$5,0,$6,$7)`, databaseUUID(source.ID), response.ID,
			response.SubmittedAt.UTC(), fingerprint[:], operationUUID, rowNumber, now); err != nil {
			return StageResult{}, mapPostgresError("save response receipt", err)
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE google_forms_sync_runs
   SET operation_import_id=$2, cursor_completed_at=$3, received_count=$4,
       staged_count=$5, duplicate_count=$6, version=version+1, updated_at=$7
 WHERE id=$1 AND state='RUNNING'`, databaseUUID(run.ID), operationUUID, optionalTime(cursor),
		received, len(selected), duplicates, now); err != nil {
		return StageResult{}, fmt.Errorf("finish response staging: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return StageResult{}, fmt.Errorf("commit response staging: %w", err)
	}
	result.Import = &operations.Import{ID: operationID, ActorUserID: source.OwnerUserID,
		Module: source.Module, SourceKind: operations.SourceGoogleForms, State: operations.ImportMapping,
		Stage: operations.StageMap, Version: 1, SelectedSheetIndex: integerPointer(0), ExpiresAt: expiresAt,
		CreatedAt: now, UpdatedAt: now}
	return result, nil
}

func loadQuestionsTx(ctx context.Context, tx pgx.Tx, sourceID Identifier) ([]Question, error) {
	rows, err := tx.Query(ctx, `SELECT question_id, position, title, answer_kind,
       required, supported, unsupported_code, question_fingerprint, target_field
  FROM google_forms_questions WHERE source_id=$1 ORDER BY position`, databaseUUID(sourceID))
	if err != nil {
		return nil, fmt.Errorf("load response questions: %w", err)
	}
	defer rows.Close()
	result := make([]Question, 0)
	for rows.Next() {
		var value Question
		var unsupported, target pgtype.Text
		var fingerprint []byte
		if err := rows.Scan(&value.ID, &value.Position, &value.Title, &value.AnswerKind,
			&value.Required, &value.Supported, &unsupported, &fingerprint, &target); err != nil {
			return nil, fmt.Errorf("scan response question: %w", err)
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
	return result, rows.Err()
}

func responseCell(response Response, question Question) (string, operations.ValueKind, string) {
	answers := response.Answers[question.ID]
	if len(answers) > 1 {
		return strings.Join(answers, "; "), operations.ValueText, "google_forms_multiple_answers"
	}
	if len(answers) == 0 || strings.TrimSpace(answers[0]) == "" {
		if question.Required {
			return "", operations.ValueEmpty, "google_forms_required_answer_missing"
		}
		return "", operations.ValueEmpty, ""
	}
	raw := answers[0]
	switch question.AnswerKind {
	case AnswerDate:
		return raw, operations.ValueDate, ""
	case AnswerScale:
		return raw, operations.ValueNumber, ""
	default:
		return raw, operations.ValueText, ""
	}
}

func truncateText(value string, maximum int) string {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	runes := []rune(value)
	return string(runes[:maximum])
}

func integerPointer(value int) *int { return &value }

func (store *PostgresStore) CompleteSync(ctx context.Context, id Identifier, cursor *time.Time, nextPageToken string, received, staged, duplicates int, now time.Time) (SyncRun, error) {
	if len(nextPageToken) > 2048 {
		return SyncRun{}, ErrInvalidInput
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return SyncRun{}, fmt.Errorf("begin sync completion: %w", err)
	}
	defer tx.Rollback(ctx)
	run, err := scanSyncRun(tx.QueryRow(ctx, `UPDATE google_forms_sync_runs
   SET state='COMPLETED', cursor_completed_at=$2, received_count=$3,
       staged_count=$4, duplicate_count=$5, completed_at=$6,
       error_code=NULL, version=version+1, updated_at=$6
 WHERE id=$1 AND state='RUNNING'
RETURNING id, source_id, owner_user_id, actor_user_id, trigger_kind,
          state, idempotency_key, operation_import_id, cursor_started_at,
          cursor_completed_at, received_count, staged_count, duplicate_count,
          river_job_id, error_code, started_at, completed_at, version, created_at, updated_at`,
		databaseUUID(id), optionalTime(cursor), received, staged, duplicates, now))
	if errors.Is(err, pgx.ErrNoRows) {
		return SyncRun{}, ErrInvalidState
	}
	if err != nil {
		return SyncRun{}, fmt.Errorf("complete sync run: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE google_forms_sources
	   SET cursor_submitted_at=CASE
         WHEN $2::timestamptz IS NULL THEN cursor_submitted_at
	         WHEN cursor_submitted_at IS NULL OR cursor_submitted_at<$2 THEN $2
	         ELSE cursor_submitted_at
	       END,
	       response_page_token=NULLIF($3,''),
	       page_token_cursor_started_at=CASE
	         WHEN $3='' THEN NULL
	         ELSE COALESCE(page_token_cursor_started_at,$4)
	       END,
	       last_synced_at=$5,
	       next_sync_at=CASE WHEN state='ACTIVE' AND sync_mode='POLL'
	         THEN $5+make_interval(secs=>poll_interval_seconds) ELSE NULL END,
	       error_code=NULL, version=version+1, updated_at=$5
	 WHERE id=$1`, databaseUUID(run.SourceID), optionalTime(cursor), nextPageToken,
		optionalTime(run.CursorStartedAt), now); err != nil {
		return SyncRun{}, fmt.Errorf("advance source cursor: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return SyncRun{}, fmt.Errorf("commit sync completion: %w", err)
	}
	return run, nil
}

func (store *PostgresStore) FailSync(ctx context.Context, id Identifier, code string, now time.Time) (bool, error) {
	command, err := store.pool.Exec(ctx, `UPDATE google_forms_sync_runs
   SET state='FAILED', error_code=$2, completed_at=$3,
       version=version+1, updated_at=$3
 WHERE id=$1 AND state IN ('QUEUED','RUNNING')`, databaseUUID(id), code, now)
	if err != nil {
		return false, fmt.Errorf("fail sync run: %w", err)
	}
	return command.RowsAffected() == 1, nil
}

func (store *PostgresStore) CancelSync(ctx context.Context, id Identifier, ownerID auth.Identifier, version int64, now time.Time) (SyncRun, error) {
	value, err := scanSyncRun(store.pool.QueryRow(ctx, `UPDATE google_forms_sync_runs
   SET state='CANCELLED', completed_at=$4, error_code=NULL,
       version=version+1, updated_at=$4
 WHERE id=$1 AND owner_user_id=$2 AND version=$3 AND state IN ('QUEUED','RUNNING')
RETURNING id, source_id, owner_user_id, actor_user_id, trigger_kind,
          state, idempotency_key, operation_import_id, cursor_started_at,
          cursor_completed_at, received_count, staged_count, duplicate_count,
          river_job_id, error_code, started_at, completed_at, version, created_at, updated_at`,
		databaseUUID(id), authDatabaseUUID(ownerID), version, now))
	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		if queryErr := store.pool.QueryRow(ctx, `SELECT EXISTS (
  SELECT 1 FROM google_forms_sync_runs WHERE id=$1 AND owner_user_id=$2
)`, databaseUUID(id), authDatabaseUUID(ownerID)).Scan(&exists); queryErr != nil {
			return SyncRun{}, fmt.Errorf("classify sync cancellation: %w", queryErr)
		}
		if !exists {
			return SyncRun{}, ErrNotFound
		}
		return SyncRun{}, ErrConflict
	}
	if err != nil {
		return SyncRun{}, fmt.Errorf("cancel sync: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) ListSyncRuns(ctx context.Context, ownerID auth.Identifier, sourceID *Identifier, limit, offset int) (SyncRunPage, error) {
	if limit < 1 || limit > 100 || offset < 0 {
		return SyncRunPage{}, ErrInvalidInput
	}
	filter := ` WHERE owner_user_id=$1`
	arguments := []any{authDatabaseUUID(ownerID)}
	if sourceID != nil {
		filter += ` AND source_id=$2`
		arguments = append(arguments, databaseUUID(*sourceID))
	}
	var total int
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM google_forms_sync_runs`+filter, arguments...).Scan(&total); err != nil {
		return SyncRunPage{}, fmt.Errorf("count sync runs: %w", err)
	}
	limitPosition := len(arguments) + 1
	offsetPosition := len(arguments) + 2
	query := syncRunSelect + filter + fmt.Sprintf(` ORDER BY created_at DESC, id DESC LIMIT $%d OFFSET $%d`, limitPosition, offsetPosition)
	arguments = append(arguments, limit, offset)
	rows, err := store.pool.Query(ctx, query, arguments...)
	if err != nil {
		return SyncRunPage{}, fmt.Errorf("list sync runs: %w", err)
	}
	defer rows.Close()
	page := SyncRunPage{Total: total}
	for rows.Next() {
		value, scanErr := scanSyncRun(rows)
		if scanErr != nil {
			return SyncRunPage{}, fmt.Errorf("scan sync run: %w", scanErr)
		}
		page.Runs = append(page.Runs, value)
	}
	if err := rows.Err(); err != nil {
		return SyncRunPage{}, fmt.Errorf("iterate sync runs: %w", err)
	}
	return page, nil
}

func (store *PostgresStore) GetActor(ctx context.Context, id auth.Identifier) (auth.Session, error) {
	value, err := scanActor(store.pool.QueryRow(ctx, `SELECT id, github_user_id, github_login, display_name,
       avatar_url, role, active FROM app_users WHERE id=$1`, authDatabaseUUID(id)))
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.Session{}, ErrNotFound
	}
	if err != nil {
		return auth.Session{}, fmt.Errorf("get sync actor: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) RecordAuditEvent(ctx context.Context, value AuditEvent) error {
	if value.ID.IsZero() || value.EventType == "" || value.Outcome == "" {
		return ErrInvalidInput
	}
	_, err := store.pool.Exec(ctx, `INSERT INTO google_forms_audit_events (
  id, actor_user_id, connection_id, source_id, sync_run_id, event_type,
  outcome, affected_count, request_id, created_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, databaseUUID(value.ID),
		optionalAuthDatabaseUUID(value.ActorUserID), optionalIdentifierUUID(value.ConnectionID),
		optionalIdentifierUUID(value.SourceID), optionalIdentifierUUID(value.SyncRunID),
		value.EventType, value.Outcome, value.AffectedCount, value.RequestID, value.CreatedAt)
	if err != nil {
		return mapPostgresError("record google forms audit", err)
	}
	return nil
}

func optionalIdentifierUUID(value *Identifier) pgtype.UUID {
	if value == nil {
		return pgtype.UUID{}
	}
	return databaseUUID(*value)
}
