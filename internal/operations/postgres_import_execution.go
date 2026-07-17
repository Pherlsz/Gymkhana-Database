package operations

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/customdata"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

func (store *PostgresStore) SavePreview(ctx context.Context, id Identifier, actorID auth.Identifier, version int64, preview []Row, unresolved, validationErrors int, now time.Time) (Import, error) {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return Import{}, fmt.Errorf("begin preview save: %w", err)
	}
	defer tx.Rollback(ctx)
	var currentVersion int64
	var state ImportState
	var expiresAt time.Time
	if err := tx.QueryRow(ctx, `SELECT version, state, expires_at FROM operation_imports
 WHERE id=$1 AND actor_user_id=$2 FOR UPDATE`, databaseUUID(id), authDatabaseUUID(actorID)).Scan(&currentVersion, &state, &expiresAt); errors.Is(err, pgx.ErrNoRows) {
		return Import{}, ErrNotFound
	} else if err != nil {
		return Import{}, fmt.Errorf("lock import preview: %w", err)
	}
	if currentVersion != version {
		return Import{}, ErrConflict
	}
	if !expiresAt.After(now) {
		return Import{}, ErrExpired
	}
	if state != ImportMapping && state != ImportPreviewReady && state != ImportDecisionsRequired && state != ImportReady {
		return Import{}, ErrInvalidState
	}
	if _, err := tx.Exec(ctx, `DELETE FROM operation_import_outcomes WHERE import_id=$1 AND outcome='CONFLICTED'`, databaseUUID(id)); err != nil {
		return Import{}, fmt.Errorf("clear stale import conflicts: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE operation_import_rows
   SET proposed_action=NULL, decision=NULL, target_id=NULL, target_version=NULL,
       validation_error_count=0, decision_required=false, source_fingerprint=NULL
 WHERE import_id=$1`, databaseUUID(id)); err != nil {
		return Import{}, fmt.Errorf("clear previous preview: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE operation_import_cells SET validation_code=NULL WHERE import_id=$1`, databaseUUID(id)); err != nil {
		return Import{}, fmt.Errorf("clear previous cell validation: %w", err)
	}
	for _, row := range preview {
		var target any
		if row.TargetID != nil {
			target = databaseUUID(*row.TargetID)
		}
		var targetVersion any
		if row.TargetVersion > 0 {
			targetVersion = row.TargetVersion
		}
		command, err := tx.Exec(ctx, `UPDATE operation_import_rows
   SET proposed_action=$4, decision=$5, target_id=$6, target_version=$7,
       validation_error_count=$8, decision_required=$9, source_fingerprint=$10
 WHERE import_id=$1 AND sheet_index=$2 AND row_number=$3`,
			databaseUUID(id), row.SheetIndex, row.RowNumber, nullableAction(row.ProposedAction),
			nullableAction(row.Decision), target, targetVersion, row.ValidationErrorCount,
			row.DecisionRequired, row.SourceFingerprint[:],
		)
		if err != nil {
			return Import{}, mapPostgresError("save preview row", err)
		}
		if command.RowsAffected() != 1 {
			return Import{}, ErrStalePreview
		}
		for _, cell := range row.Cells {
			if cell.ValidationCode == "" {
				continue
			}
			if _, err := tx.Exec(ctx, `UPDATE operation_import_cells SET validation_code=$5
 WHERE import_id=$1 AND sheet_index=$2 AND row_number=$3 AND source_column=$4`, databaseUUID(id), row.SheetIndex, row.RowNumber, cell.SourceColumn, cell.ValidationCode); err != nil {
				return Import{}, fmt.Errorf("save cell validation: %w", err)
			}
		}
	}
	nextState := ImportReady
	if validationErrors > 0 {
		nextState = ImportPreviewReady
	} else if unresolved > 0 {
		nextState = ImportDecisionsRequired
	}
	if _, err := tx.Exec(ctx, `UPDATE operation_imports
   SET state=$2, stage='PREVIEW', unresolved_count=$3,
       validation_error_count=$4, conflicted_count=0, version=version+1, updated_at=$5
 WHERE id=$1`, databaseUUID(id), nextState, unresolved, validationErrors, now); err != nil {
		return Import{}, fmt.Errorf("finish import preview: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Import{}, fmt.Errorf("commit import preview: %w", err)
	}
	return store.GetImport(ctx, id, actorID)
}

func nullableAction(value Action) any {
	if value == "" {
		return nil
	}
	return value
}

func (store *PostgresStore) SaveDecisions(ctx context.Context, id Identifier, actorID auth.Identifier, version int64, decisions []DecisionInput, now time.Time) (Import, error) {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return Import{}, fmt.Errorf("begin decisions save: %w", err)
	}
	defer tx.Rollback(ctx)
	var currentVersion int64
	var state ImportState
	var selected int
	var expiresAt time.Time
	if err := tx.QueryRow(ctx, `SELECT version, state, selected_sheet_index, expires_at
 FROM operation_imports WHERE id=$1 AND actor_user_id=$2 FOR UPDATE`, databaseUUID(id), authDatabaseUUID(actorID)).Scan(&currentVersion, &state, &selected, &expiresAt); errors.Is(err, pgx.ErrNoRows) {
		return Import{}, ErrNotFound
	} else if err != nil {
		return Import{}, fmt.Errorf("lock import decisions: %w", err)
	}
	if currentVersion != version {
		return Import{}, ErrConflict
	}
	if !expiresAt.After(now) {
		return Import{}, ErrExpired
	}
	if state != ImportDecisionsRequired {
		return Import{}, ErrInvalidState
	}
	seen := make(map[int]struct{}, len(decisions))
	for _, decision := range decisions {
		if _, exists := seen[decision.RowNumber]; exists {
			return Import{}, ErrInvalidInput
		}
		seen[decision.RowNumber] = struct{}{}
		if decision.Action != ActionUpdate && decision.Action != ActionLink && decision.Action != ActionSkip {
			return Import{}, ErrInvalidInput
		}
		var target any
		var targetVersion any
		if decision.Action == ActionUpdate || decision.Action == ActionLink {
			if decision.TargetID == nil || decision.Version <= 0 {
				return Import{}, ErrInvalidInput
			}
			target = databaseUUID(*decision.TargetID)
			targetVersion = decision.Version
		}
		command, err := tx.Exec(ctx, `UPDATE operation_import_rows
   SET decision=$4, target_id=$5, target_version=$6
 WHERE import_id=$1 AND sheet_index=$2 AND row_number=$3
   AND decision_required=true AND validation_error_count=0`, databaseUUID(id), selected, decision.RowNumber, decision.Action, target, targetVersion)
		if err != nil {
			return Import{}, mapPostgresError("save import decision", err)
		}
		if command.RowsAffected() != 1 {
			return Import{}, ErrStalePreview
		}
	}
	var unresolved int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM operation_import_rows
 WHERE import_id=$1 AND sheet_index=$2 AND decision_required=true AND decision IS NULL`, databaseUUID(id), selected).Scan(&unresolved); err != nil {
		return Import{}, fmt.Errorf("count unresolved import decisions: %w", err)
	}
	nextState := ImportDecisionsRequired
	if unresolved == 0 {
		nextState = ImportReady
	}
	if _, err := tx.Exec(ctx, `UPDATE operation_imports
   SET state=$2, unresolved_count=$3, version=version+1, updated_at=$4
 WHERE id=$1`, databaseUUID(id), nextState, unresolved, now); err != nil {
		return Import{}, fmt.Errorf("finish import decisions: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Import{}, fmt.Errorf("commit import decisions: %w", err)
	}
	return store.GetImport(ctx, id, actorID)
}

func (store *PostgresStore) QueueImport(ctx context.Context, id Identifier, actorID auth.Identifier, version, jobID int64, now time.Time) (Import, error) {
	value, err := scanImport(store.pool.QueryRow(ctx, `UPDATE operation_imports
   SET state='QUEUED', stage='EXECUTE', river_job_id=$4,
       version=version+1, updated_at=$5
 WHERE id=$1 AND actor_user_id=$2 AND version=$3 AND state='READY'
   AND unresolved_count=0 AND validation_error_count=0 AND expires_at>$5
RETURNING id, actor_user_id, module, source_kind, original_filename,
          declared_size, actual_size, content_sha256, object_key, idempotency_key,
          state, stage, selected_sheet_index, mapping_version, unresolved_count,
          validation_error_count, inserted_count, updated_count, linked_count, skipped_count,
          errored_count, conflicted_count, river_job_id, error_code, expires_at,
          cancelled_at, completed_at, version, created_at, updated_at`, databaseUUID(id), authDatabaseUUID(actorID), version, jobID, now))
	if errors.Is(err, pgx.ErrNoRows) {
		return Import{}, store.classifyImportVersionWrite(ctx, id, actorID, version, now)
	}
	if err != nil {
		return Import{}, fmt.Errorf("queue import: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) BeginImport(ctx context.Context, id Identifier, now time.Time) (Import, error) {
	value, err := scanImport(store.pool.QueryRow(ctx, `UPDATE operation_imports
   SET state='RUNNING', stage='EXECUTE', version=version+1, updated_at=$2
 WHERE id=$1 AND state IN ('QUEUED','RUNNING') AND expires_at>$2
RETURNING id, actor_user_id, module, source_kind, original_filename,
          declared_size, actual_size, content_sha256, object_key, idempotency_key,
          state, stage, selected_sheet_index, mapping_version, unresolved_count,
          validation_error_count, inserted_count, updated_count, linked_count, skipped_count,
          errored_count, conflicted_count, river_job_id, error_code, expires_at,
          cancelled_at, completed_at, version, created_at, updated_at`, databaseUUID(id), now))
	if errors.Is(err, pgx.ErrNoRows) {
		value, getErr := store.getImportTrusted(ctx, id)
		if getErr != nil {
			return Import{}, getErr
		}
		if value.State == ImportCancelled {
			return Import{}, ErrCancelled
		}
		if value.State == ImportCompleted {
			return value, nil
		}
		if !value.ExpiresAt.After(now) {
			return Import{}, ErrExpired
		}
		return Import{}, ErrInvalidState
	}
	if err != nil {
		return Import{}, fmt.Errorf("begin import execution: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) ListPendingRows(ctx context.Context, id Identifier, limit int) ([]MappedRow, error) {
	if limit < 1 || limit > MaximumBatchSize {
		return nil, ErrInvalidInput
	}
	return store.loadRows(ctx, id, false, true, limit)
}

func (store *PostgresStore) ApplyRow(ctx context.Context, imported Import, row MappedRow, mutation Mutation, generatedID Identifier, requestID string, now time.Time) (Outcome, error) {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return Outcome{}, fmt.Errorf("begin imported row: %w", err)
	}
	defer tx.Rollback(ctx)
	existing, err := scanOutcome(tx.QueryRow(ctx, `SELECT outcome, target_id, error_code, committed_at
 FROM operation_import_outcomes WHERE import_id=$1 AND sheet_index=$2 AND row_number=$3 FOR UPDATE`, databaseUUID(imported.ID), row.Row.SheetIndex, row.Row.RowNumber))
	if err == nil {
		return existing, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Outcome{}, fmt.Errorf("find imported row outcome: %w", err)
	}
	var state ImportState
	if err := tx.QueryRow(ctx, `SELECT state FROM operation_imports WHERE id=$1 FOR UPDATE`, databaseUUID(imported.ID)).Scan(&state); err != nil {
		return Outcome{}, fmt.Errorf("lock import execution: %w", err)
	}
	if state != ImportRunning {
		if state == ImportCancelled {
			return Outcome{}, ErrCancelled
		}
		return Outcome{}, ErrInvalidState
	}
	var actorActive bool
	var actorRole auth.Role
	if err := tx.QueryRow(ctx, `SELECT active, role FROM app_users WHERE id=$1`, authDatabaseUUID(imported.ActorUserID)).Scan(&actorActive, &actorRole); errors.Is(err, pgx.ErrNoRows) {
		return Outcome{}, ErrForbidden
	} else if err != nil {
		return Outcome{}, fmt.Errorf("authorize imported row: %w", err)
	}
	if !canImport(auth.Session{User: auth.User{ID: imported.ActorUserID, Role: actorRole, Active: actorActive}}, imported.Module) {
		return Outcome{}, ErrForbidden
	}
	action := row.Row.ProposedAction
	if row.Row.Decision != "" {
		action = row.Row.Decision
	}
	outcome := Outcome{CommittedAt: now}
	var targetID Identifier
	switch action {
	case ActionSkip:
		outcome.Kind = OutcomeSkipped
	case ActionLink:
		if row.Row.TargetID == nil || row.Row.TargetVersion <= 0 {
			return Outcome{}, ErrInvalidInput
		}
		if err := validateCanonicalLink(ctx, tx, imported.Module, *row.Row.TargetID, row.Row.TargetVersion); err != nil {
			if isConflictError(err) || errors.Is(err, ErrNotFound) {
				outcome.Kind, outcome.ErrorCode = OutcomeConflicted, "conflict"
			} else {
				outcome.Kind, outcome.ErrorCode = OutcomeErrored, "mutation_failed"
			}
			if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
				return Outcome{}, fmt.Errorf("rollback failed imported link: %w", rollbackErr)
			}
			return store.recordFailedOutcome(ctx, imported.ID, row.Row, outcome, now)
		}
		targetID = *row.Row.TargetID
		outcome.Kind = OutcomeLinked
		outcome.TargetID = &targetID
	case ActionCreate, ActionUpdate:
		targetID, err = applyCanonicalMutation(ctx, tx, imported, row.Row, mutation, generatedID, requestID, now, action)
		if err != nil {
			if isConflictError(err) {
				outcome.Kind, outcome.ErrorCode = OutcomeConflicted, "conflict"
			} else {
				outcome.Kind, outcome.ErrorCode = OutcomeErrored, "mutation_failed"
			}
			if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
				return Outcome{}, fmt.Errorf("rollback failed imported row: %w", rollbackErr)
			}
			return store.recordFailedOutcome(ctx, imported.ID, row.Row, outcome, now)
		}
		if action == ActionCreate {
			outcome.Kind = OutcomeInserted
		} else {
			outcome.Kind = OutcomeUpdated
		}
		outcome.TargetID = &targetID
	default:
		return Outcome{}, ErrInvalidState
	}
	if _, err := tx.Exec(ctx, `INSERT INTO operation_import_outcomes (
  import_id, sheet_index, row_number, outcome, target_id, error_code, committed_at
) VALUES ($1,$2,$3,$4,$5,$6,$7)`, databaseUUID(imported.ID), row.Row.SheetIndex, row.Row.RowNumber, outcome.Kind, optionalIdentifier(outcome.TargetID), nullableString(outcome.ErrorCode), now); err != nil {
		return Outcome{}, mapPostgresError("record import outcome", err)
	}
	if _, err := tx.Exec(ctx, counterUpdateSQL(outcome.Kind), databaseUUID(imported.ID), now); err != nil {
		return Outcome{}, fmt.Errorf("increment import outcome: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Outcome{}, fmt.Errorf("commit imported row: %w", err)
	}
	return outcome, nil
}

func validateCanonicalLink(ctx context.Context, tx pgx.Tx, module Module, id Identifier, version int64) error {
	if id.IsZero() || version <= 0 {
		return ErrInvalidInput
	}
	var exists bool
	var err error
	switch module {
	case ModuleProfiles:
		err = tx.QueryRow(ctx, `SELECT true FROM profiles WHERE id=$1 AND version=$2`, databaseUUID(id), version).Scan(&exists)
	case ModuleDocuments:
		err = tx.QueryRow(ctx, `SELECT true FROM documents WHERE id=$1 AND version=$2`, databaseUUID(id), version).Scan(&exists)
	case ModuleBills:
		err = tx.QueryRow(ctx, `SELECT true FROM bills WHERE id=$1 AND version=$2`, databaseUUID(id), version).Scan(&exists)
	default:
		return ErrInvalidInput
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrConflict
	}
	if err != nil {
		return fmt.Errorf("validate canonical link: %w", err)
	}
	if !exists {
		return ErrConflict
	}
	return nil
}

func applyCanonicalMutation(ctx context.Context, tx pgx.Tx, imported Import, row Row, mutation Mutation, generatedID Identifier, requestID string, now time.Time, action Action) (Identifier, error) {
	if mutation.Module != imported.Module {
		return Identifier{}, ErrInvalidInput
	}
	targetID := generatedID
	if action == ActionUpdate {
		if row.TargetID == nil || row.TargetVersion <= 0 {
			return Identifier{}, ErrInvalidInput
		}
		targetID = *row.TargetID
	}
	switch imported.Module {
	case ModuleProfiles:
		if mutation.Profile == nil {
			return Identifier{}, ErrInvalidInput
		}
		values := mutation.Profile
		if action == ActionCreate {
			_, err := tx.Exec(ctx, `INSERT INTO profiles (
  id, full_name, social_name, cpf, email, mobile_phone, landline_phone,
  address_street, address_number, address_complement, address_neighborhood,
  address_city, address_state, address_postal_code, notes, created_at, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$16)`,
				databaseUUID(targetID), values.FullName, nullableString(values.SocialName), nullableString(values.CPF),
				nullableString(values.Email), nullableString(values.MobilePhone), nullableString(values.LandlinePhone),
				nullableString(values.Address.Street), nullableString(values.Address.Number), nullableString(values.Address.Complement),
				nullableString(values.Address.Neighborhood), nullableString(values.Address.City), nullableString(values.Address.State),
				nullableString(values.Address.PostalCode), nullableString(values.Notes), now)
			if err != nil {
				return Identifier{}, err
			}
		} else {
			command, err := tx.Exec(ctx, `UPDATE profiles SET
  full_name=$3, social_name=$4, cpf=$5, email=$6, mobile_phone=$7,
  landline_phone=$8, address_street=$9, address_number=$10,
  address_complement=$11, address_neighborhood=$12, address_city=$13,
  address_state=$14, address_postal_code=$15, notes=$16,
  version=version+1, updated_at=$17
 WHERE id=$1 AND version=$2`, databaseUUID(targetID), row.TargetVersion, values.FullName,
				nullableString(values.SocialName), nullableString(values.CPF), nullableString(values.Email),
				nullableString(values.MobilePhone), nullableString(values.LandlinePhone), nullableString(values.Address.Street),
				nullableString(values.Address.Number), nullableString(values.Address.Complement), nullableString(values.Address.Neighborhood),
				nullableString(values.Address.City), nullableString(values.Address.State), nullableString(values.Address.PostalCode),
				nullableString(values.Notes), now)
			if err != nil {
				return Identifier{}, err
			}
			if command.RowsAffected() != 1 {
				return Identifier{}, ErrConflict
			}
		}
		if err := applyCustomValueMutations(ctx, tx, imported.Module, targetID, mutation.CustomValues, now); err != nil {
			return Identifier{}, err
		}
		auditID, err := NewIdentifier()
		if err != nil {
			return Identifier{}, err
		}
		eventType := "PROFILE_CREATED"
		if action == ActionUpdate {
			eventType = "PROFILE_UPDATED"
		}
		_, err = tx.Exec(ctx, `INSERT INTO profile_audit_events (
  id, actor_user_id, profile_id, event_type, outcome, request_id, occurred_at
) VALUES ($1,$2,$3,$4,'SUCCESS',$5,$6)`, databaseUUID(auditID), authDatabaseUUID(imported.ActorUserID), databaseUUID(targetID), eventType, requestID, now)
		return targetID, err
	case ModuleDocuments:
		if mutation.Document == nil {
			return Identifier{}, ErrInvalidInput
		}
		values := mutation.Document
		if action == ActionCreate {
			command, err := tx.Exec(ctx, `INSERT INTO documents (
  id, owner_profile_id, document_type_id, identifier_value, uniqueness_policy,
  document_date, notes, record_state, created_at, updated_at
) SELECT $1,$2,t.id,$4,t.uniqueness_policy,$5,$6,$7,$8,$8
    FROM document_types t WHERE t.id=$3 AND t.active=true`, databaseUUID(targetID), databaseUUID(Identifier(values.OwnerProfileID)), databaseUUID(Identifier(values.TypeID)), values.Identifier, nullableDate(values.DocumentDate), nullableString(values.Notes), values.RecordState, now)
			if err != nil {
				return Identifier{}, err
			}
			if command.RowsAffected() != 1 {
				return Identifier{}, ErrConflict
			}
		} else {
			command, err := tx.Exec(ctx, `UPDATE documents d SET
  owner_profile_id=$3, document_type_id=t.id, identifier_value=$5,
  uniqueness_policy=t.uniqueness_policy, document_date=$6, notes=$7,
  record_state=$8, version=d.version+1, updated_at=$9
 FROM document_types t
 WHERE d.id=$1 AND d.version=$2 AND t.id=$4 AND t.active=true`, databaseUUID(targetID), row.TargetVersion, databaseUUID(Identifier(values.OwnerProfileID)), databaseUUID(Identifier(values.TypeID)), values.Identifier, nullableDate(values.DocumentDate), nullableString(values.Notes), values.RecordState, now)
			if err != nil {
				return Identifier{}, err
			}
			if command.RowsAffected() != 1 {
				return Identifier{}, ErrConflict
			}
		}
		if err := applyCustomValueMutations(ctx, tx, imported.Module, targetID, mutation.CustomValues, now); err != nil {
			return Identifier{}, err
		}
		auditID, err := NewIdentifier()
		if err != nil {
			return Identifier{}, err
		}
		eventType := "DOCUMENT_CREATED"
		if action == ActionUpdate {
			eventType = "DOCUMENT_UPDATED"
		}
		_, err = tx.Exec(ctx, `INSERT INTO document_audit_events (
  id, actor_user_id, document_id, document_type_id, event_type, outcome, request_id, occurred_at
) VALUES ($1,$2,$3,$4,$5,'SUCCESS',$6,$7)`, databaseUUID(auditID), authDatabaseUUID(imported.ActorUserID), databaseUUID(targetID), databaseUUID(Identifier(values.TypeID)), eventType, requestID, now)
		return targetID, err
	case ModuleBills:
		if mutation.Bill == nil {
			return Identifier{}, ErrInvalidInput
		}
		values := mutation.Bill
		if action == ActionCreate {
			command, err := tx.Exec(ctx, `INSERT INTO bills (
  id, owner_profile_id, bill_type_id, printed_holder_name, printed_address,
  reference_value, competence, amount, currency, notes, record_state, created_at, updated_at
) SELECT $1,$2,t.id,$4,$5,$6,$7,$8,$9,$10,$11,$12,$12
    FROM bill_types t WHERE t.id=$3 AND t.active=true`, databaseUUID(targetID), databaseUUID(Identifier(values.OwnerProfileID)), databaseUUID(Identifier(values.TypeID)), nullableString(values.PrintedHolderName), nullableString(values.PrintedAddress), nullableString(values.Reference), nullableString(values.Competence), nullableDecimal(values.Amount), nullableString(values.Currency), nullableString(values.Notes), values.RecordState, now)
			if err != nil {
				return Identifier{}, err
			}
			if command.RowsAffected() != 1 {
				return Identifier{}, ErrConflict
			}
		} else {
			command, err := tx.Exec(ctx, `UPDATE bills b SET
  owner_profile_id=$3, bill_type_id=t.id, printed_holder_name=$5,
  printed_address=$6, reference_value=$7, competence=$8, amount=$9,
  currency=$10, notes=$11, record_state=$12, version=b.version+1, updated_at=$13
 FROM bill_types t
 WHERE b.id=$1 AND b.version=$2 AND t.id=$4 AND t.active=true`, databaseUUID(targetID), row.TargetVersion, databaseUUID(Identifier(values.OwnerProfileID)), databaseUUID(Identifier(values.TypeID)), nullableString(values.PrintedHolderName), nullableString(values.PrintedAddress), nullableString(values.Reference), nullableString(values.Competence), nullableDecimal(values.Amount), nullableString(values.Currency), nullableString(values.Notes), values.RecordState, now)
			if err != nil {
				return Identifier{}, err
			}
			if command.RowsAffected() != 1 {
				return Identifier{}, ErrConflict
			}
		}
		if err := applyCustomValueMutations(ctx, tx, imported.Module, targetID, mutation.CustomValues, now); err != nil {
			return Identifier{}, err
		}
		auditID, err := NewIdentifier()
		if err != nil {
			return Identifier{}, err
		}
		eventType := "BILL_CREATED"
		if action == ActionUpdate {
			eventType = "BILL_UPDATED"
		}
		_, err = tx.Exec(ctx, `INSERT INTO bill_audit_events (
  id, actor_user_id, bill_id, bill_type_id, event_type, outcome, request_id, occurred_at
) VALUES ($1,$2,$3,$4,$5,'SUCCESS',$6,$7)`, databaseUUID(auditID), authDatabaseUUID(imported.ActorUserID), databaseUUID(targetID), databaseUUID(Identifier(values.TypeID)), eventType, requestID, now)
		return targetID, err
	default:
		return Identifier{}, ErrInvalidInput
	}
}

func applyCustomValueMutations(ctx context.Context, tx pgx.Tx, module Module, targetID Identifier, values []CustomValueMutation, now time.Time) error {
	if len(values) == 0 {
		return nil
	}
	if module != ModuleProfiles {
		return ErrInvalidMapping
	}
	for _, value := range values {
		if value.Definition.Values.TargetKind != customdata.TargetProfile || value.Value.FieldDefinitionID != value.Definition.ID {
			return ErrInvalidMapping
		}
		fieldID := pgtype.UUID{Bytes: [16]byte(value.Definition.ID), Valid: true}
		if _, err := tx.Exec(ctx, `DELETE FROM custom_field_values WHERE profile_id=$1 AND field_definition_id=$2`, databaseUUID(targetID), fieldID); err != nil {
			return fmt.Errorf("clear imported custom field value: %w", err)
		}
		if !operationCustomValuePresent(value.Value) {
			continue
		}
		id, err := NewIdentifier()
		if err != nil {
			return err
		}
		var textValue, integerValue, decimalValue, booleanValue, civilDate, civilMonth any
		switch value.Value.Kind {
		case customdata.FieldText, customdata.FieldLongText, customdata.FieldEmail, customdata.FieldPhone:
			textValue = value.Value.Text
		case customdata.FieldInteger:
			integerValue = *value.Value.Integer
		case customdata.FieldDecimal:
			decimalValue = value.Value.Decimal
		case customdata.FieldBoolean:
			booleanValue = *value.Value.Boolean
		case customdata.FieldCivilDate:
			parsed, err := time.Parse("2006-01-02", value.Value.CivilDate)
			if err != nil {
				return ErrInvalidInput
			}
			civilDate = parsed
		case customdata.FieldCivilMonth:
			civilMonth = value.Value.CivilMonth
		default:
			return ErrInvalidMapping
		}
		if _, err := tx.Exec(ctx, `INSERT INTO custom_field_values (
  id, field_definition_id, profile_id, field_kind, text_value, integer_value,
  decimal_value, boolean_value, civil_date_value, civil_month_value, created_at, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$11)`, databaseUUID(id), fieldID,
			databaseUUID(targetID), value.Value.Kind, textValue, integerValue, decimalValue,
			booleanValue, civilDate, civilMonth, now); err != nil {
			return mapPostgresError("store imported custom field value", err)
		}
	}
	return nil
}

func operationCustomValuePresent(value customdata.ValueInput) bool {
	switch value.Kind {
	case customdata.FieldText, customdata.FieldLongText, customdata.FieldEmail, customdata.FieldPhone:
		return value.Text != ""
	case customdata.FieldInteger:
		return value.Integer != nil
	case customdata.FieldDecimal:
		return value.Decimal != ""
	case customdata.FieldBoolean:
		return value.Boolean != nil
	case customdata.FieldCivilDate:
		return value.CivilDate != ""
	case customdata.FieldCivilMonth:
		return value.CivilMonth != ""
	default:
		return false
	}
}

func scanOutcome(row rowScanner) (Outcome, error) {
	var value Outcome
	var targetID pgtype.UUID
	var errorCode pgtype.Text
	if err := row.Scan(&value.Kind, &targetID, &errorCode, &value.CommittedAt); err != nil {
		return Outcome{}, err
	}
	if targetID.Valid {
		identifier := identifierFromUUID(targetID)
		value.TargetID = &identifier
	}
	if errorCode.Valid {
		value.ErrorCode = errorCode.String
	}
	return value, nil
}

func (store *PostgresStore) recordFailedOutcome(ctx context.Context, importID Identifier, row Row, outcome Outcome, now time.Time) (Outcome, error) {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return Outcome{}, fmt.Errorf("begin failed outcome: %w", err)
	}
	defer tx.Rollback(ctx)
	var state ImportState
	if err := tx.QueryRow(ctx, `SELECT state FROM operation_imports WHERE id=$1 FOR UPDATE`, databaseUUID(importID)).Scan(&state); err != nil {
		return Outcome{}, fmt.Errorf("lock failed import outcome: %w", err)
	}
	if state == ImportCancelled {
		return Outcome{}, ErrCancelled
	}
	if state != ImportRunning {
		return Outcome{}, ErrInvalidState
	}
	existing, err := scanOutcome(tx.QueryRow(ctx, `SELECT outcome, target_id, error_code, committed_at
 FROM operation_import_outcomes WHERE import_id=$1 AND sheet_index=$2 AND row_number=$3 FOR UPDATE`, databaseUUID(importID), row.SheetIndex, row.RowNumber))
	if err == nil {
		return existing, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Outcome{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO operation_import_outcomes (
  import_id, sheet_index, row_number, outcome, error_code, committed_at
) VALUES ($1,$2,$3,$4,$5,$6)`, databaseUUID(importID), row.SheetIndex, row.RowNumber, outcome.Kind, outcome.ErrorCode, now); err != nil {
		return Outcome{}, mapPostgresError("record failed import outcome", err)
	}
	if _, err := tx.Exec(ctx, counterUpdateSQL(outcome.Kind), databaseUUID(importID), now); err != nil {
		return Outcome{}, fmt.Errorf("increment failed import outcome: %w", err)
	}
	return outcome, tx.Commit(ctx)
}

func counterUpdateSQL(kind OutcomeKind) string {
	column := "errored_count"
	switch kind {
	case OutcomeInserted:
		column = "inserted_count"
	case OutcomeUpdated:
		column = "updated_count"
	case OutcomeLinked:
		column = "linked_count"
	case OutcomeSkipped:
		column = "skipped_count"
	case OutcomeConflicted:
		column = "conflicted_count"
	}
	return `UPDATE operation_imports SET ` + column + `=` + column + `+1, updated_at=$2 WHERE id=$1`
}

func isConflictError(err error) bool {
	if errors.Is(err, ErrConflict) {
		return true
	}
	var postgresErr *pgconn.PgError
	return errors.As(err, &postgresErr) && (postgresErr.Code == "23505" || postgresErr.Code == "23503" || postgresErr.Code == "23514")
}

func (store *PostgresStore) ReopenImportForPreview(ctx context.Context, id Identifier, now time.Time) (Import, error) {
	value, err := scanImport(store.pool.QueryRow(ctx, `UPDATE operation_imports
   SET state='PREVIEW_READY', stage='PREVIEW', unresolved_count=0,
       version=version+1, updated_at=$2
 WHERE id=$1 AND state='RUNNING'
RETURNING id, actor_user_id, module, source_kind, original_filename,
          declared_size, actual_size, content_sha256, object_key, idempotency_key,
          state, stage, selected_sheet_index, mapping_version, unresolved_count,
          validation_error_count, inserted_count, updated_count, linked_count, skipped_count,
          errored_count, conflicted_count, river_job_id, error_code, expires_at,
          cancelled_at, completed_at, version, created_at, updated_at`, databaseUUID(id), now))
	if errors.Is(err, pgx.ErrNoRows) {
		value, getErr := store.getImportTrusted(ctx, id)
		if getErr == nil && value.State == ImportCancelled {
			return Import{}, ErrCancelled
		}
		return Import{}, ErrInvalidState
	}
	if err != nil {
		return Import{}, fmt.Errorf("reopen import preview: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) CompleteImport(ctx context.Context, id Identifier, now time.Time) (Import, error) {
	value, err := scanImport(store.pool.QueryRow(ctx, `UPDATE operation_imports
   SET state='COMPLETED', stage='REPORT', completed_at=$2,
       version=version+1, updated_at=$2
 WHERE id=$1 AND state='RUNNING'
RETURNING id, actor_user_id, module, source_kind, original_filename,
          declared_size, actual_size, content_sha256, object_key, idempotency_key,
          state, stage, selected_sheet_index, mapping_version, unresolved_count,
          validation_error_count, inserted_count, updated_count, linked_count, skipped_count,
          errored_count, conflicted_count, river_job_id, error_code, expires_at,
          cancelled_at, completed_at, version, created_at, updated_at`, databaseUUID(id), now))
	if errors.Is(err, pgx.ErrNoRows) {
		value, getErr := store.getImportTrusted(ctx, id)
		if getErr == nil && value.State == ImportCompleted {
			return value, nil
		}
		return Import{}, ErrInvalidState
	}
	if err != nil {
		return Import{}, fmt.Errorf("complete import: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) FailImport(ctx context.Context, id Identifier, expected ImportState, code string, now time.Time) (bool, error) {
	command, err := store.pool.Exec(ctx, `UPDATE operation_imports
	   SET state='FAILED', error_code=$2, version=version+1, updated_at=$3
 WHERE id=$1 AND state=$4`, databaseUUID(id), code, now, expected)
	if err != nil {
		return false, fmt.Errorf("fail import: %w", err)
	}
	return command.RowsAffected() == 1, nil
}

func (store *PostgresStore) CancelImport(ctx context.Context, id Identifier, actorID auth.Identifier, version int64, now time.Time) (Import, error) {
	value, err := scanImport(store.pool.QueryRow(ctx, `UPDATE operation_imports
   SET state='CANCELLED', cancelled_at=$4, version=version+1, updated_at=$4
 WHERE id=$1 AND actor_user_id=$2 AND version=$3
   AND state NOT IN ('COMPLETED','CANCELLED','EXPIRED')
RETURNING id, actor_user_id, module, source_kind, original_filename,
          declared_size, actual_size, content_sha256, object_key, idempotency_key,
          state, stage, selected_sheet_index, mapping_version, unresolved_count,
          validation_error_count, inserted_count, updated_count, linked_count, skipped_count,
          errored_count, conflicted_count, river_job_id, error_code, expires_at,
          cancelled_at, completed_at, version, created_at, updated_at`, databaseUUID(id), authDatabaseUUID(actorID), version, now))
	if errors.Is(err, pgx.ErrNoRows) {
		return Import{}, store.classifyImportVersionWrite(ctx, id, actorID, version, now)
	}
	if err != nil {
		return Import{}, fmt.Errorf("cancel import: %w", err)
	}
	return value, nil
}

func optionalIdentifier(value *Identifier) any {
	if value == nil {
		return nil
	}
	return databaseUUID(*value)
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableDate(value string) any    { return nullableString(value) }
func nullableDecimal(value string) any { return nullableString(value) }
