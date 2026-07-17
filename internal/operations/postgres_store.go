package operations

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

const importSelect = `SELECT id, actor_user_id, module, source_kind, original_filename,
       declared_size, actual_size, content_sha256, object_key, idempotency_key,
       state, stage, selected_sheet_index, mapping_version, unresolved_count,
       validation_error_count, inserted_count, updated_count, linked_count, skipped_count,
       errored_count, conflicted_count, river_job_id, error_code, expires_at,
       cancelled_at, completed_at, version, created_at, updated_at
  FROM operation_imports`

const exportSelect = `SELECT id, actor_user_id, module, idempotency_key, state, object_key,
       filename, row_count, byte_size, content_sha256, river_job_id, error_code,
       expires_at, completed_at, version, created_at, updated_at
  FROM operation_exports`

type rowScanner interface {
	Scan(...any) error
}

func scanImport(row rowScanner) (Import, error) {
	var value Import
	var id, actorID pgtype.UUID
	var originalFilename, objectKey pgtype.Text
	var declaredSize, actualSize, riverJobID pgtype.Int8
	var selectedSheet pgtype.Int4
	var sha []byte
	var errorCode pgtype.Text
	var cancelledAt, completedAt pgtype.Timestamptz
	if err := row.Scan(
		&id, &actorID, &value.Module, &value.SourceKind, &originalFilename,
		&declaredSize, &actualSize, &sha, &objectKey, &value.IdempotencyKey,
		&value.State, &value.Stage, &selectedSheet, &value.MappingVersion, &value.UnresolvedCount,
		&value.ValidationErrorCount, &value.InsertedCount, &value.UpdatedCount, &value.LinkedCount, &value.SkippedCount,
		&value.ErroredCount, &value.ConflictedCount, &riverJobID, &errorCode, &value.ExpiresAt,
		&cancelledAt, &completedAt, &value.Version, &value.CreatedAt, &value.UpdatedAt,
	); err != nil {
		return Import{}, err
	}
	value.ID = identifierFromUUID(id)
	value.ActorUserID = authIdentifierFromUUID(actorID)
	if originalFilename.Valid {
		value.OriginalFilename = originalFilename.String
	}
	if declaredSize.Valid {
		value.DeclaredSize = declaredSize.Int64
	}
	if actualSize.Valid {
		value.ActualSize = actualSize.Int64
	}
	copy(value.ContentSHA256[:], sha)
	if objectKey.Valid {
		value.ObjectKey = objectKey.String
	}
	if selectedSheet.Valid {
		index := int(selectedSheet.Int32)
		value.SelectedSheetIndex = &index
	}
	if riverJobID.Valid {
		value.RiverJobID = riverJobID.Int64
	}
	if errorCode.Valid {
		value.ErrorCode = errorCode.String
	}
	if cancelledAt.Valid {
		value.CancelledAt = &cancelledAt.Time
	}
	if completedAt.Valid {
		value.CompletedAt = &completedAt.Time
	}
	return value, nil
}

func scanExport(row rowScanner) (Export, error) {
	var value Export
	var id, actorID pgtype.UUID
	var byteSize, riverJobID pgtype.Int8
	var sha []byte
	var errorCode pgtype.Text
	var completedAt pgtype.Timestamptz
	if err := row.Scan(
		&id, &actorID, &value.Module, &value.IdempotencyKey, &value.State, &value.ObjectKey,
		&value.Filename, &value.RowCount, &byteSize, &sha, &riverJobID, &errorCode,
		&value.ExpiresAt, &completedAt, &value.Version, &value.CreatedAt, &value.UpdatedAt,
	); err != nil {
		return Export{}, err
	}
	value.ID = identifierFromUUID(id)
	value.ActorUserID = authIdentifierFromUUID(actorID)
	if byteSize.Valid {
		value.ByteSize = byteSize.Int64
	}
	copy(value.ContentSHA256[:], sha)
	if riverJobID.Valid {
		value.RiverJobID = riverJobID.Int64
	}
	if errorCode.Valid {
		value.ErrorCode = errorCode.String
	}
	if completedAt.Valid {
		value.CompletedAt = &completedAt.Time
	}
	return value, nil
}

func (store *PostgresStore) CreateImport(ctx context.Context, value Import, limits Limits) (Import, error) {
	if store == nil || store.pool == nil || value.ID.IsZero() || !value.Module.Valid() || value.ActorUserID == (auth.Identifier{}) {
		return Import{}, ErrInvalidInput
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Import{}, fmt.Errorf("begin import creation: %w", err)
	}
	defer tx.Rollback(ctx)
	actorID := authDatabaseUUID(value.ActorUserID)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 8091))`, actorID); err != nil {
		return Import{}, fmt.Errorf("lock import actor: %w", err)
	}
	existing, err := scanImport(tx.QueryRow(ctx, importSelect+` WHERE actor_user_id=$1 AND idempotency_key=$2`, actorID, value.IdempotencyKey))
	if err == nil {
		return existing, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Import{}, fmt.Errorf("find idempotent import: %w", err)
	}
	if err := reserveOperationLimit(ctx, tx, actorID, limits); err != nil {
		return Import{}, err
	}
	created, err := scanImport(tx.QueryRow(ctx, `INSERT INTO operation_imports (
  id, actor_user_id, module, source_kind, original_filename, declared_size,
  object_key, idempotency_key, state, stage, expires_at, created_at, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$12)
RETURNING id, actor_user_id, module, source_kind, original_filename,
          declared_size, actual_size, content_sha256, object_key, idempotency_key,
          state, stage, selected_sheet_index, mapping_version, unresolved_count,
          validation_error_count, inserted_count, updated_count, linked_count, skipped_count,
          errored_count, conflicted_count, river_job_id, error_code, expires_at,
          cancelled_at, completed_at, version, created_at, updated_at`,
		databaseUUID(value.ID), actorID, value.Module, value.SourceKind, value.OriginalFilename,
		value.DeclaredSize, value.ObjectKey, value.IdempotencyKey, value.State, value.Stage,
		value.ExpiresAt, value.CreatedAt,
	))
	if err != nil {
		return Import{}, mapPostgresError("create import", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Import{}, fmt.Errorf("commit import creation: %w", err)
	}
	return created, nil
}

func reserveOperationLimit(ctx context.Context, tx pgx.Tx, actorID pgtype.UUID, limits Limits) error {
	if limits.MaximumRequests <= 0 || limits.MaximumActive <= 0 || limits.WindowStart.IsZero() {
		return ErrInvalidInput
	}
	var count int
	if err := tx.QueryRow(ctx, `INSERT INTO operation_rate_limits (
  actor_user_id, window_started_at, request_count, updated_at
) VALUES ($1,$2,1,now())
ON CONFLICT (actor_user_id) DO UPDATE SET
  window_started_at = CASE WHEN operation_rate_limits.window_started_at < $2 THEN $2 ELSE operation_rate_limits.window_started_at END,
  request_count = CASE WHEN operation_rate_limits.window_started_at < $2 THEN 1 ELSE operation_rate_limits.request_count + 1 END,
  updated_at = now()
RETURNING request_count`, actorID, limits.WindowStart).Scan(&count); err != nil {
		return fmt.Errorf("reserve operation rate limit: %w", err)
	}
	if count > limits.MaximumRequests {
		return ErrRateLimited
	}
	var active int
	if err := tx.QueryRow(ctx, `SELECT
  (SELECT count(*) FROM operation_imports WHERE actor_user_id=$1 AND state IN ('UPLOADING','UPLOADED','PARSING','MAPPING','PREVIEW_READY','DECISIONS_REQUIRED','READY','QUEUED','RUNNING')) +
  (SELECT count(*) FROM operation_exports WHERE actor_user_id=$1 AND state IN ('QUEUED','RUNNING'))`, actorID).Scan(&active); err != nil {
		return fmt.Errorf("count active operations: %w", err)
	}
	if active >= limits.MaximumActive {
		return ErrQuotaExceeded
	}
	return nil
}

func (store *PostgresStore) ConfirmImport(ctx context.Context, id Identifier, actorID auth.Identifier, actualSize int64, sha [32]byte, jobID int64, now time.Time) (Import, error) {
	value, err := scanImport(store.pool.QueryRow(ctx, `UPDATE operation_imports
   SET actual_size=$3, content_sha256=$4, state='PARSING', stage='PARSE',
       river_job_id=$5, version=version+1, updated_at=$6
 WHERE id=$1 AND actor_user_id=$2 AND state='UPLOADING' AND expires_at>$6
   AND declared_size=$3
RETURNING id, actor_user_id, module, source_kind, original_filename,
          declared_size, actual_size, content_sha256, object_key, idempotency_key,
          state, stage, selected_sheet_index, mapping_version, unresolved_count,
          validation_error_count, inserted_count, updated_count, linked_count, skipped_count,
          errored_count, conflicted_count, river_job_id, error_code, expires_at,
          cancelled_at, completed_at, version, created_at, updated_at`,
		databaseUUID(id), authDatabaseUUID(actorID), actualSize, sha[:], nullablePositiveInt64(jobID), now,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return Import{}, store.classifyImportWrite(ctx, id, actorID, now)
	}
	if err != nil {
		return Import{}, fmt.Errorf("confirm import: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) GetImport(ctx context.Context, id Identifier, actorID auth.Identifier) (Import, error) {
	value, err := scanImport(store.pool.QueryRow(ctx, importSelect+` WHERE id=$1 AND actor_user_id=$2`, databaseUUID(id), authDatabaseUUID(actorID)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Import{}, ErrNotFound
	}
	if err != nil {
		return Import{}, fmt.Errorf("get import: %w", err)
	}
	if err := store.loadImportDetails(ctx, &value); err != nil {
		return Import{}, err
	}
	return value, nil
}

func (store *PostgresStore) getImportTrusted(ctx context.Context, id Identifier) (Import, error) {
	value, err := scanImport(store.pool.QueryRow(ctx, importSelect+` WHERE id=$1`, databaseUUID(id)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Import{}, ErrNotFound
	}
	if err != nil {
		return Import{}, fmt.Errorf("get trusted import: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) GetImportForWorker(ctx context.Context, id Identifier) (Import, error) {
	return store.getImportTrusted(ctx, id)
}

func (store *PostgresStore) GetReport(ctx context.Context, id Identifier, actorID auth.Identifier) (Report, error) {
	value, err := scanImport(store.pool.QueryRow(ctx, importSelect+` WHERE id=$1 AND actor_user_id=$2`, databaseUUID(id), authDatabaseUUID(actorID)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Report{}, ErrNotFound
	}
	if err != nil {
		return Report{}, fmt.Errorf("get import report: %w", err)
	}
	report := Report{
		ImportID: value.ID, State: value.State, Inserted: value.InsertedCount,
		Updated: value.UpdatedCount, Linked: value.LinkedCount, Skipped: value.SkippedCount, Errored: value.ErroredCount,
		Conflicted: value.ConflictedCount, Unresolved: value.UnresolvedCount,
		ValidationErrors: value.ValidationErrorCount,
	}
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM operation_import_rows WHERE import_id=$1 AND decision IS NOT NULL`, databaseUUID(id)).Scan(&report.Decisions); err != nil {
		return Report{}, fmt.Errorf("count import report decisions: %w", err)
	}
	rows, err := store.pool.Query(ctx, `SELECT staged.sheet_index, staged.row_number, result.outcome,
       COALESCE(result.error_code, CASE WHEN staged.validation_error_count>0 THEN 'validation_failed' END)
  FROM operation_import_rows staged
  LEFT JOIN operation_import_outcomes result
    ON result.import_id=staged.import_id AND result.sheet_index=staged.sheet_index AND result.row_number=staged.row_number
 WHERE staged.import_id=$1 AND (result.import_id IS NOT NULL OR staged.validation_error_count>0)
 ORDER BY staged.sheet_index, staged.row_number
 LIMIT $2`, databaseUUID(id), MaximumReportRows)
	if err != nil {
		return Report{}, fmt.Errorf("list import report rows: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var row ReportRow
		var outcome, errorCode pgtype.Text
		if err := rows.Scan(&row.SheetIndex, &row.RowNumber, &outcome, &errorCode); err != nil {
			return Report{}, fmt.Errorf("scan import report row: %w", err)
		}
		if outcome.Valid {
			row.Outcome = OutcomeKind(outcome.String)
		}
		if errorCode.Valid {
			row.ErrorCode = errorCode.String
		}
		report.Rows = append(report.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return Report{}, fmt.Errorf("iterate import report rows: %w", err)
	}
	return report, nil
}

func (store *PostgresStore) loadImportDetails(ctx context.Context, value *Import) error {
	sheets, err := store.pool.Query(ctx, `SELECT sheet_index, name, row_count, column_count
  FROM operation_import_sheets WHERE import_id=$1 ORDER BY sheet_index`, databaseUUID(value.ID))
	if err != nil {
		return fmt.Errorf("list import sheets: %w", err)
	}
	for sheets.Next() {
		var sheet Sheet
		if err := sheets.Scan(&sheet.Index, &sheet.Name, &sheet.RowCount, &sheet.ColumnCount); err != nil {
			sheets.Close()
			return fmt.Errorf("scan import sheet: %w", err)
		}
		value.Sheets = append(value.Sheets, sheet)
	}
	if err := sheets.Err(); err != nil {
		sheets.Close()
		return fmt.Errorf("iterate import sheets: %w", err)
	}
	sheets.Close()
	if value.SelectedSheetIndex == nil {
		return nil
	}
	columns, err := store.pool.Query(ctx, `SELECT sheet_index, source_column, source_header, target_field
  FROM operation_import_columns WHERE import_id=$1 AND sheet_index=$2 ORDER BY source_column`, databaseUUID(value.ID), *value.SelectedSheetIndex)
	if err != nil {
		return fmt.Errorf("list import columns: %w", err)
	}
	for columns.Next() {
		var column Column
		var target pgtype.Text
		if err := columns.Scan(&column.SheetIndex, &column.SourceColumn, &column.SourceHeader, &target); err != nil {
			columns.Close()
			return fmt.Errorf("scan import column: %w", err)
		}
		if target.Valid {
			column.TargetField = target.String
		}
		value.Columns = append(value.Columns, column)
	}
	if err := columns.Err(); err != nil {
		columns.Close()
		return fmt.Errorf("iterate import columns: %w", err)
	}
	columns.Close()
	preview, err := store.loadRows(ctx, value.ID, true, false, MaximumPreviewRows)
	if err != nil {
		return err
	}
	for _, row := range preview {
		value.Preview = append(value.Preview, row.Row)
	}
	return nil
}

func (store *PostgresStore) ListImports(ctx context.Context, actorID auth.Identifier, options ListOptions) (ImportPage, error) {
	options = normalizeListOptions(options)
	if options.Limit == 0 {
		return ImportPage{}, ErrInvalidInput
	}
	var total int64
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM operation_imports WHERE actor_user_id=$1`, authDatabaseUUID(actorID)).Scan(&total); err != nil {
		return ImportPage{}, fmt.Errorf("count imports: %w", err)
	}
	rows, err := store.pool.Query(ctx, importSelect+` WHERE actor_user_id=$1 ORDER BY created_at DESC, id DESC LIMIT $2 OFFSET $3`, authDatabaseUUID(actorID), options.Limit, options.Offset)
	if err != nil {
		return ImportPage{}, fmt.Errorf("list imports: %w", err)
	}
	defer rows.Close()
	page := ImportPage{Total: total, Limit: options.Limit, Offset: options.Offset}
	for rows.Next() {
		value, scanErr := scanImport(rows)
		if scanErr != nil {
			return ImportPage{}, fmt.Errorf("scan import: %w", scanErr)
		}
		page.Imports = append(page.Imports, value)
	}
	if err := rows.Err(); err != nil {
		return ImportPage{}, fmt.Errorf("iterate imports: %w", err)
	}
	return page, nil
}

func (store *PostgresStore) StageWorkbook(ctx context.Context, id Identifier, workbook Workbook, now time.Time) error {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin workbook staging: %w", err)
	}
	defer tx.Rollback(ctx)
	var state ImportState
	var expiresAt time.Time
	if err := tx.QueryRow(ctx, `SELECT state, expires_at FROM operation_imports WHERE id=$1 FOR UPDATE`, databaseUUID(id)).Scan(&state, &expiresAt); errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return fmt.Errorf("lock import for staging: %w", err)
	}
	if !expiresAt.After(now) {
		return ErrExpired
	}
	if state != ImportParsing {
		if state == ImportMapping || state == ImportPreviewReady || state == ImportDecisionsRequired || state == ImportReady || state == ImportQueued || state == ImportRunning || state == ImportCompleted {
			return tx.Commit(ctx)
		}
		if state == ImportCancelled {
			return ErrCancelled
		}
		return ErrInvalidState
	}
	if _, err := tx.Exec(ctx, `DELETE FROM operation_import_sheets WHERE import_id=$1`, databaseUUID(id)); err != nil {
		return fmt.Errorf("clear import staging: %w", err)
	}
	for _, sheet := range workbook.Sheets {
		headers, columnCount, err := workbookHeaders(sheet)
		if err != nil {
			return err
		}
		rowCount := 0
		for _, row := range sheet.Rows {
			if row.Number > 1 {
				rowCount++
			}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO operation_import_sheets (import_id, sheet_index, name, row_count, column_count)
VALUES ($1,$2,$3,$4,$5)`, databaseUUID(id), sheet.Index, sheet.Name, rowCount, columnCount); err != nil {
			return mapPostgresError("insert import sheet", err)
		}
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"operation_import_columns"},
			[]string{"import_id", "sheet_index", "source_column", "source_header"},
			pgx.CopyFromSlice(len(headers), func(column int) ([]any, error) {
				return []any{databaseUUID(id), sheet.Index, column, headers[column]}, nil
			})); err != nil {
			return mapPostgresError("stage import columns", err)
		}
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"operation_import_rows"},
			[]string{"import_id", "sheet_index", "row_number"},
			&workbookRowCopySource{importID: id, sheet: sheet}); err != nil {
			return mapPostgresError("stage import rows", err)
		}
		cellSource := &workbookCellCopySource{importID: id, sheet: sheet, columnCount: columnCount}
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"operation_import_cells"},
			[]string{"import_id", "sheet_index", "row_number", "source_column", "raw_value", "value_kind", "formula_present"},
			cellSource); err != nil {
			return mapPostgresError("stage import cells", err)
		}
	}
	command, err := tx.Exec(ctx, `UPDATE operation_imports
   SET state='MAPPING', stage='MAP', selected_sheet_index=NULL,
       mapping_version=0, river_job_id=NULL, error_code=NULL,
       version=version+1, updated_at=$2
 WHERE id=$1 AND state='PARSING'`, databaseUUID(id), now)
	if err != nil {
		return fmt.Errorf("finish workbook staging: %w", err)
	}
	if command.RowsAffected() != 1 {
		return ErrInvalidState
	}
	return tx.Commit(ctx)
}

type workbookRowCopySource struct {
	importID Identifier
	sheet    WorkbookSheet
	position int
	current  WorkbookRow
}

func (source *workbookRowCopySource) Next() bool {
	for source.position < len(source.sheet.Rows) {
		source.current = source.sheet.Rows[source.position]
		source.position++
		if source.current.Number > 1 {
			return true
		}
	}
	return false
}

func (source *workbookRowCopySource) Values() ([]any, error) {
	return []any{databaseUUID(source.importID), source.sheet.Index, source.current.Number}, nil
}

func (*workbookRowCopySource) Err() error { return nil }

type workbookCellCopySource struct {
	importID    Identifier
	sheet       WorkbookSheet
	columnCount int
	row         int
	cell        int
	currentRow  WorkbookRow
	current     WorkbookCell
	err         error
}

func (source *workbookCellCopySource) Next() bool {
	for source.row < len(source.sheet.Rows) {
		if source.cell == 0 {
			source.currentRow = source.sheet.Rows[source.row]
		}
		if source.currentRow.Number <= 1 || source.cell >= len(source.currentRow.Cells) {
			source.row++
			source.cell = 0
			continue
		}
		source.current = source.currentRow.Cells[source.cell]
		source.cell++
		if source.current.Column >= source.columnCount {
			source.err = ErrUnsupportedWorkbook
			return false
		}
		return true
	}
	return false
}

func (source *workbookCellCopySource) Values() ([]any, error) {
	return []any{databaseUUID(source.importID), source.sheet.Index, source.currentRow.Number,
		source.current.Column, source.current.Value, source.current.Kind, source.current.FormulaPresent}, nil
}

func (source *workbookCellCopySource) Err() error { return source.err }

func workbookHeaders(sheet WorkbookSheet) ([]string, int, error) {
	var header *WorkbookRow
	maximumColumn := -1
	for index := range sheet.Rows {
		row := &sheet.Rows[index]
		if row.Number == 1 {
			header = row
		}
		for _, cell := range row.Cells {
			if cell.Column > maximumColumn {
				maximumColumn = cell.Column
			}
		}
	}
	if header == nil || maximumColumn < 0 || maximumColumn >= MaximumColumns {
		return nil, 0, ErrUnsupportedWorkbook
	}
	headers := make([]string, maximumColumn+1)
	for _, cell := range header.Cells {
		if cell.FormulaPresent {
			return nil, 0, ErrUnsupportedWorkbook
		}
		headers[cell.Column] = strings.TrimSpace(cell.Value)
	}
	seen := make(map[string]struct{}, len(headers))
	for _, value := range headers {
		folded := strings.ToLower(value)
		if value == "" || len(value) > 500 {
			return nil, 0, ErrUnsupportedWorkbook
		}
		if _, exists := seen[folded]; exists {
			return nil, 0, ErrUnsupportedWorkbook
		}
		seen[folded] = struct{}{}
	}
	return headers, len(headers), nil
}

func (store *PostgresStore) SelectSheet(ctx context.Context, id Identifier, actorID auth.Identifier, version int64, sheetIndex int, now time.Time) (Import, error) {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return Import{}, fmt.Errorf("begin import sheet selection: %w", err)
	}
	defer tx.Rollback(ctx)
	value, err := scanImport(tx.QueryRow(ctx, `UPDATE operation_imports i
   SET selected_sheet_index=$4, state='MAPPING', stage='MAP', mapping_version=mapping_version+1,
       unresolved_count=0, validation_error_count=0, inserted_count=0, updated_count=0,
       linked_count=0, skipped_count=0, errored_count=0, conflicted_count=0,
       version=version+1, updated_at=$5
 WHERE i.id=$1 AND i.actor_user_id=$2 AND i.version=$3
   AND i.state IN ('MAPPING','PREVIEW_READY','DECISIONS_REQUIRED','READY')
	AND i.expires_at>$5
   AND EXISTS (SELECT 1 FROM operation_import_sheets s WHERE s.import_id=i.id AND s.sheet_index=$4)
   AND NOT EXISTS (
     SELECT 1 FROM operation_import_outcomes outcome
      WHERE outcome.import_id=i.id AND outcome.outcome<>'CONFLICTED'
   )
RETURNING id, actor_user_id, module, source_kind, original_filename,
          declared_size, actual_size, content_sha256, object_key, idempotency_key,
          state, stage, selected_sheet_index, mapping_version, unresolved_count,
          validation_error_count, inserted_count, updated_count, linked_count, skipped_count,
          errored_count, conflicted_count, river_job_id, error_code, expires_at,
          cancelled_at, completed_at, version, created_at, updated_at`,
		databaseUUID(id), authDatabaseUUID(actorID), version, sheetIndex, now,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		_ = tx.Rollback(ctx)
		return Import{}, store.classifyImportVersionWrite(ctx, id, actorID, version, now)
	}
	if err != nil {
		return Import{}, fmt.Errorf("select import sheet: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE operation_import_columns SET target_field=NULL WHERE import_id=$1`, databaseUUID(id)); err != nil {
		return Import{}, fmt.Errorf("clear mapping after sheet selection: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM operation_import_outcomes WHERE import_id=$1`, databaseUUID(id)); err != nil {
		return Import{}, fmt.Errorf("clear conflicts after sheet selection: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE operation_import_rows
   SET proposed_action=NULL, decision=NULL, target_id=NULL, target_version=NULL,
       validation_error_count=0, decision_required=false, source_fingerprint=NULL
 WHERE import_id=$1`, databaseUUID(id)); err != nil {
		return Import{}, fmt.Errorf("clear preview after sheet selection: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE operation_import_cells SET validation_code=source_validation_code WHERE import_id=$1`, databaseUUID(id)); err != nil {
		return Import{}, fmt.Errorf("clear validation after sheet selection: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Import{}, fmt.Errorf("commit import sheet selection: %w", err)
	}
	return store.GetImport(ctx, value.ID, actorID)
}

func (store *PostgresStore) SaveMapping(ctx context.Context, id Identifier, actorID auth.Identifier, version int64, mapping []MappingInput, now time.Time) (Import, error) {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return Import{}, fmt.Errorf("begin mapping save: %w", err)
	}
	defer tx.Rollback(ctx)
	var selected pgtype.Int4
	var currentVersion int64
	var state ImportState
	var expiresAt time.Time
	if err := tx.QueryRow(ctx, `SELECT selected_sheet_index, version, state, expires_at
  FROM operation_imports WHERE id=$1 AND actor_user_id=$2 FOR UPDATE`, databaseUUID(id), authDatabaseUUID(actorID)).Scan(&selected, &currentVersion, &state, &expiresAt); errors.Is(err, pgx.ErrNoRows) {
		return Import{}, ErrNotFound
	} else if err != nil {
		return Import{}, fmt.Errorf("lock import mapping: %w", err)
	}
	if currentVersion != version {
		return Import{}, ErrConflict
	}
	if !expiresAt.After(now) {
		return Import{}, ErrExpired
	}
	if !selected.Valid || (state != ImportMapping && state != ImportPreviewReady && state != ImportDecisionsRequired && state != ImportReady) {
		return Import{}, ErrInvalidState
	}
	var hasCommittedOutcomes bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (
  SELECT 1 FROM operation_import_outcomes WHERE import_id=$1 AND outcome<>'CONFLICTED'
)`, databaseUUID(id)).Scan(&hasCommittedOutcomes); err != nil {
		return Import{}, fmt.Errorf("check committed import outcomes: %w", err)
	}
	if hasCommittedOutcomes {
		return Import{}, ErrInvalidState
	}
	if _, err := tx.Exec(ctx, `UPDATE operation_import_columns SET target_field=NULL WHERE import_id=$1`, databaseUUID(id)); err != nil {
		return Import{}, fmt.Errorf("clear import mapping: %w", err)
	}
	for _, item := range mapping {
		command, err := tx.Exec(ctx, `UPDATE operation_import_columns SET target_field=$4
 WHERE import_id=$1 AND sheet_index=$2 AND source_column=$3`, databaseUUID(id), selected.Int32, item.SourceColumn, item.TargetField)
		if err != nil {
			return Import{}, mapPostgresError("save import mapping item", err)
		}
		if command.RowsAffected() != 1 {
			return Import{}, ErrInvalidMapping
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM operation_import_outcomes WHERE import_id=$1`, databaseUUID(id)); err != nil {
		return Import{}, fmt.Errorf("clear import outcomes: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE operation_import_rows
   SET proposed_action=NULL, decision=NULL, target_id=NULL, target_version=NULL,
       validation_error_count=0, decision_required=false, source_fingerprint=NULL
 WHERE import_id=$1`, databaseUUID(id)); err != nil {
		return Import{}, fmt.Errorf("clear import preview rows: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE operation_import_cells SET validation_code=source_validation_code WHERE import_id=$1`, databaseUUID(id)); err != nil {
		return Import{}, fmt.Errorf("clear import validation: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE operation_imports
   SET state='MAPPING', stage='MAP', mapping_version=mapping_version+1,
       unresolved_count=0, validation_error_count=0, inserted_count=0, updated_count=0,
       linked_count=0, skipped_count=0, errored_count=0, conflicted_count=0,
       version=version+1, updated_at=$2
 WHERE id=$1`, databaseUUID(id), now); err != nil {
		return Import{}, fmt.Errorf("finish import mapping: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Import{}, fmt.Errorf("commit import mapping: %w", err)
	}
	return store.GetImport(ctx, id, actorID)
}

func (store *PostgresStore) LoadMappedRows(ctx context.Context, id Identifier) ([]MappedRow, error) {
	return store.loadRows(ctx, id, false, false, MaximumRows)
}

func (store *PostgresStore) loadRows(ctx context.Context, id Identifier, previewOnly, pendingOnly bool, limit int) ([]MappedRow, error) {
	if limit < 1 || limit > MaximumRows {
		return nil, ErrInvalidInput
	}
	query := `SELECT r.sheet_index, r.row_number, r.proposed_action, r.decision,
       r.target_id, r.target_version, r.validation_error_count, r.decision_required,
       r.source_fingerprint, o.outcome, o.target_id, o.error_code, o.committed_at
  FROM operation_import_rows r
  JOIN operation_imports i ON i.id=r.import_id AND i.selected_sheet_index=r.sheet_index
  LEFT JOIN operation_import_outcomes o ON o.import_id=r.import_id AND o.sheet_index=r.sheet_index AND o.row_number=r.row_number
 WHERE r.import_id=$1`
	if previewOnly {
		query += ` AND r.proposed_action IS NOT NULL`
	}
	if pendingOnly {
		query += ` AND o.import_id IS NULL`
	}
	if previewOnly {
		// Keep the bounded preview actionable for imports with more than 200
		// ambiguous rows. Once a window is decided, the next unresolved rows move
		// into the response instead of being hidden behind the same first page.
		query += ` ORDER BY CASE WHEN r.decision_required AND r.decision IS NULL THEN 0 ELSE 1 END, r.row_number LIMIT $2`
	} else {
		query += ` ORDER BY r.row_number LIMIT $2`
	}
	rows, err := store.pool.Query(ctx, query, databaseUUID(id), limit)
	if err != nil {
		return nil, fmt.Errorf("list mapped import rows: %w", err)
	}
	defer rows.Close()
	result := make([]MappedRow, 0)
	byNumber := make(map[int]int)
	for rows.Next() {
		var mapped MappedRow
		var proposed, decision pgtype.Text
		var targetID, outcomeTargetID pgtype.UUID
		var targetVersion pgtype.Int8
		var fingerprint []byte
		var outcomeKind, outcomeError pgtype.Text
		var committedAt pgtype.Timestamptz
		if err := rows.Scan(&mapped.Row.SheetIndex, &mapped.Row.RowNumber, &proposed, &decision,
			&targetID, &targetVersion, &mapped.Row.ValidationErrorCount, &mapped.Row.DecisionRequired,
			&fingerprint, &outcomeKind, &outcomeTargetID, &outcomeError, &committedAt); err != nil {
			return nil, fmt.Errorf("scan mapped import row: %w", err)
		}
		if proposed.Valid {
			mapped.Row.ProposedAction = Action(proposed.String)
		}
		if decision.Valid {
			mapped.Row.Decision = Action(decision.String)
		}
		if targetID.Valid {
			identifier := identifierFromUUID(targetID)
			mapped.Row.TargetID = &identifier
		}
		if targetVersion.Valid {
			mapped.Row.TargetVersion = targetVersion.Int64
		}
		copy(mapped.Row.SourceFingerprint[:], fingerprint)
		if outcomeKind.Valid {
			outcome := &Outcome{Kind: OutcomeKind(outcomeKind.String)}
			if outcomeTargetID.Valid {
				identifier := identifierFromUUID(outcomeTargetID)
				outcome.TargetID = &identifier
			}
			if outcomeError.Valid {
				outcome.ErrorCode = outcomeError.String
			}
			if committedAt.Valid {
				outcome.CommittedAt = committedAt.Time
			}
			mapped.Row.Outcome = outcome
		}
		mapped.Values = make(map[string]string)
		byNumber[mapped.Row.RowNumber] = len(result)
		result = append(result, mapped)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate mapped import rows: %w", err)
	}
	if len(result) == 0 {
		return result, nil
	}
	rowNumbers := make([]int32, 0, len(result))
	for _, mapped := range result {
		rowNumbers = append(rowNumbers, int32(mapped.Row.RowNumber))
	}
	cells, err := store.pool.Query(ctx, `SELECT c.row_number, c.source_column, c.raw_value,
	       c.value_kind, c.formula_present, c.source_validation_code, columns.target_field
  FROM operation_import_cells c
  JOIN operation_imports i ON i.id=c.import_id AND i.selected_sheet_index=c.sheet_index
  JOIN operation_import_columns columns
    ON columns.import_id=c.import_id AND columns.sheet_index=c.sheet_index AND columns.source_column=c.source_column
 WHERE c.import_id=$1 AND c.row_number=ANY($2::integer[]) AND columns.target_field IS NOT NULL
 ORDER BY c.row_number, c.source_column`, databaseUUID(id), rowNumbers)
	if err != nil {
		return nil, fmt.Errorf("list mapped import cells: %w", err)
	}
	defer cells.Close()
	for cells.Next() {
		var rowNumber, sourceColumn int
		var raw, target string
		var kind ValueKind
		var formula bool
		var validation pgtype.Text
		if err := cells.Scan(&rowNumber, &sourceColumn, &raw, &kind, &formula, &validation, &target); err != nil {
			return nil, fmt.Errorf("scan mapped import cell: %w", err)
		}
		position, ok := byNumber[rowNumber]
		if !ok {
			continue
		}
		cell := Cell{SourceColumn: sourceColumn, RawValue: raw, ValueKind: kind, FormulaPresent: formula}
		if validation.Valid {
			cell.ValidationCode = validation.String
		}
		result[position].Row.Cells = append(result[position].Row.Cells, cell)
		result[position].Values[target] = raw
	}
	if err := cells.Err(); err != nil {
		return nil, fmt.Errorf("iterate mapped import cells: %w", err)
	}
	return result, nil
}

func normalizeListOptions(options ListOptions) ListOptions {
	if options.Limit == 0 {
		options.Limit = 50
	}
	if options.Limit < 1 || options.Limit > 100 || options.Offset < 0 || options.Offset > 10_000 {
		return ListOptions{}
	}
	return options
}

func (store *PostgresStore) classifyImportWrite(ctx context.Context, id Identifier, actorID auth.Identifier, now time.Time) error {
	var owner pgtype.UUID
	var state ImportState
	var expiresAt time.Time
	err := store.pool.QueryRow(ctx, `SELECT actor_user_id, state, expires_at FROM operation_imports WHERE id=$1`, databaseUUID(id)).Scan(&owner, &state, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && authIdentifierFromUUID(owner) != actorID) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("classify import write: %w", err)
	}
	if !expiresAt.After(now) {
		return ErrExpired
	}
	return ErrInvalidState
}

func (store *PostgresStore) classifyImportVersionWrite(ctx context.Context, id Identifier, actorID auth.Identifier, version int64, now time.Time) error {
	value, err := store.GetImport(ctx, id, actorID)
	if err != nil {
		return err
	}
	if !value.ExpiresAt.After(now) {
		return ErrExpired
	}
	if value.Version != version {
		return ErrConflict
	}
	return ErrInvalidState
}

func mapPostgresError(action string, err error) error {
	if errors.Is(err, pgx.ErrTxCommitRollback) {
		return fmt.Errorf("%s: %w", action, ErrConflict)
	}
	var postgresErr *pgconn.PgError
	if errors.As(err, &postgresErr) {
		switch postgresErr.Code {
		case "23503", "23505", "23514", "40001":
			return fmt.Errorf("%s: %w", action, ErrConflict)
		}
	}
	return fmt.Errorf("%s: %w", action, err)
}

func databaseUUID(value Identifier) pgtype.UUID { return pgtype.UUID{Bytes: value, Valid: true} }
func authDatabaseUUID(value auth.Identifier) pgtype.UUID {
	return pgtype.UUID{Bytes: value, Valid: true}
}
func identifierFromUUID(value pgtype.UUID) Identifier          { return Identifier(value.Bytes) }
func authIdentifierFromUUID(value pgtype.UUID) auth.Identifier { return auth.Identifier(value.Bytes) }

func nullablePositiveInt64(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}
