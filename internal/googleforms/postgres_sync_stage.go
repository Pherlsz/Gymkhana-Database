package googleforms

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/operations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

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
	inputFingerprints := make(map[string][32]byte, len(values))
	for _, value := range values {
		if value.ID == "" || len(value.ID) > 500 || value.SubmittedAt.IsZero() {
			return StageResult{}, ErrInvalidInput
		}
		fingerprint := responseFingerprint(value)
		if previous, duplicate := inputFingerprints[value.ID]; duplicate {
			if previous != fingerprint {
				return StageResult{}, ErrResponseChanged
			}
			continue
		}
		inputFingerprints[value.ID] = fingerprint
		ids = append(ids, value.ID)
	}
	existing := make(map[string][32]byte)
	if len(ids) > 0 {
		rows, queryErr := tx.Query(ctx, `SELECT provider_response_id, response_fingerprint
  FROM google_forms_response_receipts
 WHERE source_id=$1 AND provider_response_id=ANY($2::text[])`, databaseUUID(source.ID), ids)
		if queryErr != nil {
			return StageResult{}, fmt.Errorf("load response receipts: %w", queryErr)
		}
		for rows.Next() {
			var responseID string
			var fingerprint []byte
			if scanErr := rows.Scan(&responseID, &fingerprint); scanErr != nil {
				rows.Close()
				return StageResult{}, fmt.Errorf("scan response receipt: %w", scanErr)
			}
			var digest [32]byte
			copy(digest[:], fingerprint)
			existing[responseID] = digest
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
		if fingerprint, duplicate := existing[value.ID]; duplicate {
			if fingerprint != inputFingerprints[value.ID] {
				return StageResult{}, ErrResponseChanged
			}
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
