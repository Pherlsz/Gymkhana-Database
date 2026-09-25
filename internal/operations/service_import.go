package operations

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/importcatalog"
)

func (service *Service) CreateImport(ctx context.Context, actor auth.Session, input CreateImportInput, requestID string) (UploadGrant, error) {
	if !canImport(actor, input.Module) {
		service.audit(ctx, &actor.User.ID, nil, nil, input.Module, AuditImportCreated, auth.AuditOutcomeDenied, nil, requestID)
		return UploadGrant{}, ErrForbidden
	}
	input.OriginalFilename = sanitizeFilename(input.OriginalFilename)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !validXLSXFilename(input.OriginalFilename) || input.DeclaredSize <= 0 || input.DeclaredSize > MaximumFileSize || !validIdempotencyKey(input.IdempotencyKey) {
		return UploadGrant{}, ErrInvalidInput
	}
	id, err := NewIdentifier()
	if err != nil {
		return UploadGrant{}, fmt.Errorf("generate import identifier: %w", err)
	}
	now := service.now().UTC()
	value := Import{
		ID: id, ActorUserID: actor.User.ID, Module: input.Module, SourceKind: SourceXLSX,
		OriginalFilename: input.OriginalFilename, DeclaredSize: input.DeclaredSize,
		ObjectKey: importObjectKey(id, now), IdempotencyKey: input.IdempotencyKey,
		State: ImportUploading, Stage: StageUpload, ExpiresAt: now.Add(service.importRetention),
		CreatedAt: now, UpdatedAt: now,
	}
	created, err := service.store.CreateImport(ctx, value, service.limits(now))
	if err != nil {
		service.audit(ctx, &actor.User.ID, nil, nil, input.Module, AuditImportCreated, auth.AuditOutcomeFailure, nil, requestID)
		return UploadGrant{}, err
	}
	replayed := created.ID != id
	if replayed && (created.Module != input.Module || created.OriginalFilename != input.OriginalFilename ||
		created.DeclaredSize != input.DeclaredSize || created.State != ImportUploading || !created.ExpiresAt.After(now)) {
		service.audit(ctx, &actor.User.ID, &created.ID, nil, created.Module, AuditImportCreated, auth.AuditOutcomeFailure, nil, requestID)
		return UploadGrant{}, ErrConflict
	}
	signed, err := service.objects.PresignOperationUpload(ctx, created.ObjectKey, created.DeclaredSize, service.uploadTTL)
	if err != nil {
		if !replayed {
			return UploadGrant{}, service.failImport(ctx, created, ImportUploading, "storage_unavailable", requestID, err)
		}
		return UploadGrant{}, err
	}
	service.audit(ctx, &actor.User.ID, &created.ID, nil, created.Module, AuditImportCreated, auth.AuditOutcomeSuccess, nil, requestID)
	return UploadGrant{Import: created, UploadURL: signed.URL, Method: signed.Method, Headers: signed.Headers, ExpiresAt: signed.ExpiresAt}, nil
}

func (service *Service) ConfirmImport(ctx context.Context, actor auth.Session, id Identifier, requestID string) (Import, error) {
	value, err := service.store.GetImport(ctx, id, actor.User.ID)
	if err != nil {
		return Import{}, err
	}
	if !canImport(actor, value.Module) {
		service.audit(ctx, &actor.User.ID, &id, nil, value.Module, AuditImportConfirmed, auth.AuditOutcomeDenied, nil, requestID)
		return Import{}, ErrForbidden
	}
	if !value.ExpiresAt.After(service.now().UTC()) {
		return Import{}, ErrExpired
	}
	if value.State != ImportUploading {
		return Import{}, ErrInvalidState
	}
	// ponytail: skip R2 GET here; ParseImport is the single read (hash + workbook).
	jobID, err := service.jobs.EnqueueParse(ctx, id)
	if err != nil {
		return Import{}, err
	}
	now := service.now().UTC()
	confirmed, err := service.store.ConfirmImport(ctx, id, actor.User.ID, value.DeclaredSize, [32]byte{}, jobID, now)
	if err != nil {
		return Import{}, err
	}
	service.audit(ctx, &actor.User.ID, &id, nil, value.Module, AuditImportConfirmed, auth.AuditOutcomeSuccess, nil, requestID)
	return confirmed, nil
}

func (service *Service) ParseImport(ctx context.Context, id Identifier) error {
	value, err := service.store.GetImportForWorker(ctx, id)
	if err != nil {
		return err
	}
	if value.State == ImportMapping || value.State == ImportPreviewReady || value.State == ImportDecisionsRequired || value.State == ImportReady || value.State == ImportQueued || value.State == ImportRunning || value.State == ImportCompleted {
		return nil
	}
	if value.State == ImportCancelled {
		return ErrCancelled
	}
	if value.State == ImportExpired {
		return ErrExpired
	}
	if value.State == ImportFailed {
		return ErrConflict
	}
	if value.State != ImportParsing {
		return ErrInvalidState
	}
	if !value.ExpiresAt.After(service.now().UTC()) {
		return ErrExpired
	}
	opened, err := service.objects.Open(ctx, value.ObjectKey)
	if err != nil {
		return service.failImport(ctx, value, ImportParsing, "storage_unavailable", workerRequestID("parse", id), err)
	}
	workbook, data, parseErr := ReadWorkbook(opened, value.DeclaredSize)
	closeErr := opened.Close()
	if parseErr != nil || closeErr != nil {
		return service.failImport(ctx, value, ImportParsing, workbookErrorCode(parseErr), workerRequestID("parse", id), errors.Join(parseErr, closeErr))
	}
	digest := sha256.Sum256(data)
	if value.ContentSHA256 != ([32]byte{}) && digest != value.ContentSHA256 {
		return service.failImport(ctx, value, ImportParsing, "content_changed", workerRequestID("parse", id), ErrConflict)
	}
	now := service.now().UTC()
	if err := service.store.StageWorkbook(ctx, id, workbook, now); err != nil {
		if errors.Is(err, ErrCancelled) || errors.Is(err, ErrInvalidState) {
			return err
		}
		return service.failImport(ctx, value, ImportParsing, workbookErrorCode(err), workerRequestID("parse", id), err)
	}
	if err := service.applyImportCatalog(ctx, id, value.Module, nil); err != nil {
		return service.failImport(ctx, value, ImportParsing, "catalog_mapping_failed", workerRequestID("parse", id), err)
	}
	if err := service.advanceReadyMapping(ctx, id); err != nil {
		return err
	}
	if delErr := service.objects.Delete(ctx, value.ObjectKey); delErr != nil {
		// ponytail: leftover object until expires_at cleanup; staging already won.
	}
	service.audit(ctx, &value.ActorUserID, &id, nil, value.Module, AuditImportParsed, auth.AuditOutcomeSuccess, nil, workerRequestID("parse", id))
	return nil
}

func (service *Service) GetImport(ctx context.Context, actor auth.Session, id Identifier) (Import, error) {
	if !actor.User.Active || !actor.User.Role.CanUseOperations() {
		return Import{}, ErrForbidden
	}
	return service.store.GetImport(ctx, id, actor.User.ID)
}

func (service *Service) ListImports(ctx context.Context, actor auth.Session, options ListOptions) (ImportPage, error) {
	if !actor.User.Active || !actor.User.Role.CanUseOperations() {
		return ImportPage{}, ErrForbidden
	}
	return service.store.ListImports(ctx, actor.User.ID, options)
}

func (service *Service) GetReport(ctx context.Context, actor auth.Session, id Identifier) (Report, error) {
	if !actor.User.Active || !actor.User.Role.CanUseOperations() {
		return Report{}, ErrForbidden
	}
	return service.store.GetReport(ctx, id, actor.User.ID)
}

func (service *Service) SelectSheet(ctx context.Context, actor auth.Session, id Identifier, version int64, sheetIndex int, requestID string) (Import, error) {
	value, err := service.store.GetImport(ctx, id, actor.User.ID)
	if err != nil {
		return Import{}, err
	}
	if !canImport(actor, value.Module) {
		return Import{}, ErrForbidden
	}
	if !value.ExpiresAt.After(service.now().UTC()) {
		return Import{}, ErrExpired
	}
	if _, err := service.store.SelectSheet(ctx, id, actor.User.ID, version, sheetIndex, service.now().UTC()); err != nil {
		return Import{}, err
	}
	if err := service.applyImportCatalog(ctx, id, value.Module, &sheetIndex); err != nil {
		return Import{}, err
	}
	if err := service.advanceReadyMapping(ctx, id); err != nil {
		return Import{}, err
	}
	service.audit(ctx, &actor.User.ID, &id, nil, value.Module, AuditImportMapped, auth.AuditOutcomeSuccess, nil, requestID)
	return service.store.GetImport(ctx, id, actor.User.ID)
}

func (service *Service) applyImportCatalog(ctx context.Context, id Identifier, module Module, sheetIndex *int) error {
	if service == nil || service.store == nil {
		return ErrInvalidServiceSetup
	}
	return service.store.ApplySuggestedColumnMapping(ctx, id, module, sheetIndex)
}

func (service *Service) SaveMapping(ctx context.Context, actor auth.Session, id Identifier, version int64, mapping []MappingInput, requestID string) (Import, error) {
	value, err := service.store.GetImport(ctx, id, actor.User.ID)
	if err != nil {
		return Import{}, err
	}
	if !canImport(actor, value.Module) {
		return Import{}, ErrForbidden
	}
	if !value.ExpiresAt.After(service.now().UTC()) {
		return Import{}, ErrExpired
	}
	catalog, err := service.catalogModule(ctx, actor.User.Role, value.Module)
	if err != nil {
		return Import{}, err
	}
	if err := ValidateMapping(catalog, mapping); err != nil {
		return Import{}, err
	}
	mapped, err := service.store.SaveMapping(ctx, id, actor.User.ID, version, mapping, service.now().UTC())
	if err == nil {
		service.audit(ctx, &actor.User.ID, &id, nil, value.Module, AuditImportMapped, auth.AuditOutcomeSuccess, nil, requestID)
	}
	return mapped, err
}

func ValidateMapping(catalog ModuleCatalog, mapping []MappingInput) error {
	if !catalog.ID.Valid() || !catalog.CanImport || len(mapping) == 0 || len(mapping) > MaximumColumns {
		return ErrInvalidMapping
	}
	allowed := make(map[string]Field, len(catalog.Fields))
	for _, field := range catalog.Fields {
		if field.Importable {
			allowed[field.ID] = field
		}
	}
	seenColumns := make(map[int]struct{}, len(mapping))
	seenFields := make(map[string]struct{}, len(mapping))
	for _, item := range mapping {
		field, exists := allowed[item.TargetField]
		if item.SourceColumn < 0 || item.SourceColumn >= MaximumColumns || !exists || !field.Importable {
			return ErrInvalidMapping
		}
		if _, duplicate := seenColumns[item.SourceColumn]; duplicate {
			return ErrInvalidMapping
		}
		if _, duplicate := seenFields[item.TargetField]; duplicate {
			return ErrInvalidMapping
		}
		seenColumns[item.SourceColumn] = struct{}{}
		seenFields[item.TargetField] = struct{}{}
	}
	for _, field := range catalog.Fields {
		if field.Required {
			if _, mapped := seenFields[field.ID]; !mapped {
				return ErrInvalidMapping
			}
		}
	}
	_, hasRecordID := seenFields["record_id"]
	_, hasVersion := seenFields["version"]
	if hasRecordID != hasVersion {
		return ErrInvalidMapping
	}
	if catalog.ID == ModuleBills {
		_, hasAmount := seenFields["amount"]
		_, hasCurrency := seenFields["currency"]
		if hasAmount != hasCurrency {
			return ErrInvalidMapping
		}
	}
	return nil
}

func (service *Service) Preview(ctx context.Context, actor auth.Session, id Identifier, version int64, requestID string) (Import, error) {
	value, err := service.store.GetImport(ctx, id, actor.User.ID)
	if err != nil {
		return Import{}, err
	}
	if !canImport(actor, value.Module) || value.Version != version {
		if value.Version != version {
			return Import{}, ErrConflict
		}
		return Import{}, ErrForbidden
	}
	if !value.ExpiresAt.After(service.now().UTC()) {
		return Import{}, ErrExpired
	}
	return service.saveComputedPreview(ctx, value, version, actor.User.ID, requestID)
}

func mappingFromColumns(columns []Column) []MappingInput {
	mapping := make([]MappingInput, 0, len(columns))
	for _, column := range columns {
		target := importcatalog.VisibleTarget(column.TargetField)
		if target == "" {
			continue
		}
		mapping = append(mapping, MappingInput{SourceColumn: column.SourceColumn, TargetField: target})
	}
	return mapping
}

func (service *Service) advanceReadyMapping(ctx context.Context, id Identifier) error {
	value, err := service.store.GetImportForWorker(ctx, id)
	if err != nil {
		return err
	}
	if value.SelectedSheetIndex == nil || value.State != ImportMapping {
		return nil
	}
	detailed, err := service.store.GetImport(ctx, id, value.ActorUserID)
	if err != nil {
		return err
	}
	actor, err := service.store.GetActor(ctx, detailed.ActorUserID)
	if err != nil {
		return err
	}
	catalog, err := service.catalogModule(ctx, actor.User.Role, detailed.Module)
	if err != nil {
		return err
	}
	if err := ValidateMapping(catalog, mappingFromColumns(detailed.Columns)); err != nil {
		return nil
	}
	_, err = service.saveComputedPreview(ctx, detailed, detailed.Version, detailed.ActorUserID, workerRequestID("preview", id))
	if errors.Is(err, ErrInvalidState) {
		return nil
	}
	return err
}

func (service *Service) saveComputedPreview(ctx context.Context, value Import, version int64, actorID auth.Identifier, requestID string) (Import, error) {
	customFields, err := service.store.CustomFields(ctx, value.Module)
	if err != nil {
		return Import{}, err
	}
	rows, err := service.store.LoadMappedRows(ctx, value.ID)
	if err != nil {
		return Import{}, err
	}
	if len(rows) == 0 {
		return Import{}, ErrInvalidState
	}
	preview := make([]Row, 0, len(rows))
	unresolved, validationErrors := 0, 0
	for _, mapped := range rows {
		row := mapped.Row
		row.Cells = append([]Cell(nil), mapped.Row.Cells...)
		row.SourceFingerprint = fingerprintRow(value.MappingVersion, mapped.Values)
		if row.Outcome != nil && row.Outcome.Kind != OutcomeConflicted {
			row.ProposedAction = actionFromOutcome(row.Outcome.Kind)
			row.DecisionRequired = false
			row.ValidationErrorCount = 0
			preview = append(preview, row)
			continue
		}
		row.ProposedAction = ""
		row.Decision = ""
		row.TargetID = nil
		row.TargetVersion = 0
		row.ValidationErrorCount = 0
		row.DecisionRequired = false
		row.Outcome = nil
		if sourceErrors := rowSourceErrorCount(row); sourceErrors > 0 {
			row.ProposedAction = ActionError
			row.ValidationErrorCount = sourceErrors
		} else if rowHasFormula(row) {
			row.ProposedAction = ActionError
			row.ValidationErrorCount++
			markFormulaErrors(&row)
		} else if allMappedValuesEmpty(mapped.Values) {
			row.ProposedAction = ActionSkip
		} else {
			targetID, _, action, targetErr := service.resolveTarget(ctx, value.Module, mapped.Values)
			if targetErr != nil {
				row.ProposedAction = ActionError
				row.ValidationErrorCount++
				code := "invalid_target"
				if errors.Is(targetErr, ErrAmbiguousCPF) {
					code = "cpf_ambiguous"
				}
				markFirstError(&row, code)
			} else if _, canonicalVersion, mutationErr := service.mutationForRow(ctx, value.Module, mapped.Values, customFields); mutationErr != nil {
				row.ProposedAction = ActionError
				row.ValidationErrorCount++
				markFirstError(&row, "validation_failed")
			} else {
				row.ProposedAction = action
				row.TargetID = targetID
				row.TargetVersion = canonicalVersion
				if action == ActionUpdate {
					row.DecisionRequired = true
					unresolved++
				}
			}
		}
		validationErrors += row.ValidationErrorCount
		preview = append(preview, row)
	}
	saved, err := service.store.SavePreview(ctx, value.ID, actorID, version, preview, unresolved, validationErrors, service.now().UTC())
	if err == nil {
		service.audit(ctx, &actorID, &value.ID, nil, value.Module, AuditImportPreviewed, auth.AuditOutcomeSuccess, nil, requestID)
	}
	return saved, err
}

func (service *Service) SaveDecisions(ctx context.Context, actor auth.Session, id Identifier, version int64, decisions []DecisionInput, requestID string) (Import, error) {
	value, err := service.store.GetImport(ctx, id, actor.User.ID)
	if err != nil {
		return Import{}, err
	}
	if !canImport(actor, value.Module) {
		return Import{}, ErrForbidden
	}
	if !value.ExpiresAt.After(service.now().UTC()) {
		return Import{}, ErrExpired
	}
	if len(decisions) == 0 || len(decisions) > MaximumRows {
		return Import{}, ErrInvalidInput
	}
	saved, err := service.store.SaveDecisions(ctx, id, actor.User.ID, version, decisions, service.now().UTC())
	if err == nil {
		service.audit(ctx, &actor.User.ID, &id, nil, value.Module, AuditImportDecided, auth.AuditOutcomeSuccess, nil, requestID)
	}
	return saved, err
}

func (service *Service) Execute(ctx context.Context, actor auth.Session, id Identifier, version int64, requestID string) (Import, error) {
	value, err := service.store.GetImport(ctx, id, actor.User.ID)
	if err != nil {
		return Import{}, err
	}
	if !canImport(actor, value.Module) {
		return Import{}, ErrForbidden
	}
	if !value.ExpiresAt.After(service.now().UTC()) {
		return Import{}, ErrExpired
	}
	jobID, err := service.jobs.EnqueueExecute(ctx, id)
	if err != nil {
		return Import{}, err
	}
	queued, err := service.store.QueueImport(ctx, id, actor.User.ID, version, jobID, service.now().UTC())
	if err != nil {
		return Import{}, err
	}
	service.audit(ctx, &actor.User.ID, &id, nil, value.Module, AuditImportStarted, auth.AuditOutcomeSuccess, nil, requestID)
	return queued, nil
}

func (service *Service) ExecuteImport(ctx context.Context, id Identifier) error {
	now := service.now().UTC()
	value, err := service.store.BeginImport(ctx, id, now)
	if err != nil {
		return err
	}
	if value.State == ImportCompleted {
		return nil
	}
	actor, err := service.store.GetActor(ctx, value.ActorUserID)
	if err != nil || !canImport(actor, value.Module) {
		return service.failImport(ctx, value, ImportRunning, "actor_forbidden", workerRequestID("execute", id), ErrForbidden)
	}
	customFields, err := service.store.CustomFields(ctx, value.Module)
	if err != nil {
		return service.failImport(ctx, value, ImportRunning, "load_catalog_failed", workerRequestID("execute", id), err)
	}
	for {
		rows, err := service.store.ListPendingRows(ctx, id, MaximumBatchSize)
		if err != nil {
			return service.failImport(ctx, value, ImportRunning, "load_batch_failed", workerRequestID("execute", id), err)
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			if row.Row.SourceFingerprint != fingerprintRow(value.MappingVersion, row.Values) {
				_, reopenErr := service.store.ReopenImportForPreview(ctx, id, service.now().UTC())
				return errors.Join(ErrStalePreview, reopenErr)
			}
			action := effectiveAction(row.Row)
			var mutation Mutation
			if action != ActionSkip && action != ActionLink {
				mutation, _, err = service.mutationForRow(ctx, value.Module, row.Values, customFields)
			}
			if err != nil {
				if action == ActionUpdate {
					_, reopenErr := service.store.ReopenImportForPreview(ctx, id, service.now().UTC())
					return errors.Join(ErrStalePreview, reopenErr)
				}
				return service.failImport(ctx, value, ImportRunning, "stale_validation", workerRequestID("execute", id), ErrStalePreview)
			}
			generatedID := deterministicRowIdentifier(value.ID, row.Row)
			outcome, err := service.store.ApplyRow(ctx, value, row, mutation, generatedID, workerRequestID("execute", id), service.now().UTC())
			if err != nil {
				if errors.Is(err, ErrCancelled) {
					return err
				}
				return service.failImport(ctx, value, ImportRunning, "batch_failed", workerRequestID("execute", id), err)
			}
			if outcome.Kind == OutcomeConflicted {
				_, reopenErr := service.store.ReopenImportForPreview(ctx, id, service.now().UTC())
				return errors.Join(ErrStalePreview, reopenErr)
			}
		}
	}
	completed, err := service.store.CompleteImport(ctx, id, service.now().UTC())
	if err != nil {
		return err
	}
	count := completed.InsertedCount + completed.UpdatedCount + completed.LinkedCount + completed.SkippedCount + completed.ErroredCount + completed.ConflictedCount
	service.audit(ctx, &value.ActorUserID, &id, nil, value.Module, AuditImportCompleted, auth.AuditOutcomeSuccess, &count, workerRequestID("execute", id))
	return nil
}

func (service *Service) CancelImport(ctx context.Context, actor auth.Session, id Identifier, version int64, requestID string) (Import, error) {
	value, err := service.store.GetImport(ctx, id, actor.User.ID)
	if err != nil {
		return Import{}, err
	}
	if !canImport(actor, value.Module) {
		return Import{}, ErrForbidden
	}
	cancelled, err := service.store.CancelImport(ctx, id, actor.User.ID, version, service.now().UTC())
	if err != nil {
		return Import{}, err
	}
	if value.RiverJobID > 0 {
		_ = service.jobs.Cancel(ctx, value.RiverJobID)
	}
	service.audit(ctx, &actor.User.ID, &id, nil, value.Module, AuditImportCancelled, auth.AuditOutcomeSuccess, nil, requestID)
	return cancelled, nil
}

func (service *Service) DeleteImport(ctx context.Context, actor auth.Session, id Identifier, _ string) error {
	value, err := service.store.GetImport(ctx, id, actor.User.ID)
	if err != nil {
		return err
	}
	if !canImport(actor, value.Module) {
		return ErrForbidden
	}
	if !value.State.Terminal() {
		return ErrInvalidState
	}
	if value.ObjectKey != "" {
		if err := service.objects.Delete(ctx, value.ObjectKey); err != nil {
			return err
		}
	}
	if err := service.store.DeleteImport(ctx, id, actor.User.ID); err != nil {
		return err
	}
	return nil
}

func (service *Service) failImport(ctx context.Context, value Import, expected ImportState, code, requestID string, cause error) error {
	changed, failErr := service.store.FailImport(ctx, value.ID, expected, code, service.now().UTC())
	if changed {
		service.audit(ctx, &value.ActorUserID, &value.ID, nil, value.Module, AuditImportFailed, auth.AuditOutcomeFailure, nil, requestID)
	}
	return errors.Join(cause, failErr)
}

func importObjectKey(id Identifier, now time.Time) string {
	return fmt.Sprintf("operations/imports/%04d/%02d/%s.xlsx", now.Year(), now.Month(), id.String())
}

func rowHasFormula(row Row) bool {
	for _, cell := range row.Cells {
		if cell.FormulaPresent {
			return true
		}
	}
	return false
}

func rowSourceErrorCount(row Row) int {
	count := 0
	for _, cell := range row.Cells {
		if cell.ValidationCode != "" {
			count++
		}
	}
	return count
}

func markFormulaErrors(row *Row) {
	for index := range row.Cells {
		if row.Cells[index].FormulaPresent {
			row.Cells[index].ValidationCode = "formula_not_allowed"
		}
	}
}

func markFirstError(row *Row, code string) {
	if len(row.Cells) > 0 {
		row.Cells[0].ValidationCode = code
	}
}

func allMappedValuesEmpty(values map[string]string) bool {
	for field, value := range values {
		if field != "record_id" && field != "version" && strings.TrimSpace(value) != "" {
			return false
		}
	}
	return strings.TrimSpace(values["record_id"]) == "" && strings.TrimSpace(values["version"]) == ""
}

func fingerprintRow(mappingVersion int64, values map[string]string) [32]byte {
	hash := sha256.New()
	var number [8]byte
	binary.BigEndian.PutUint64(number[:], uint64(mappingVersion))
	_, _ = hash.Write(number[:])
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		binary.BigEndian.PutUint64(number[:], uint64(len(key)))
		_, _ = hash.Write(number[:])
		_, _ = io.WriteString(hash, key)
		binary.BigEndian.PutUint64(number[:], uint64(len(values[key])))
		_, _ = hash.Write(number[:])
		_, _ = io.WriteString(hash, values[key])
	}
	var result [32]byte
	copy(result[:], hash.Sum(nil))
	return result
}

func deterministicRowIdentifier(importID Identifier, row Row) Identifier {
	hash := sha256.New()
	_, _ = io.WriteString(hash, "gymkhana-import-row-v1")
	_, _ = hash.Write(importID[:])
	var number [8]byte
	binary.BigEndian.PutUint64(number[:], uint64(row.SheetIndex))
	_, _ = hash.Write(number[:])
	binary.BigEndian.PutUint64(number[:], uint64(row.RowNumber))
	_, _ = hash.Write(number[:])
	_, _ = hash.Write(row.SourceFingerprint[:])
	digest := hash.Sum(nil)
	var result Identifier
	copy(result[:], digest[:16])
	result[6] = (result[6] & 0x0f) | 0x50
	result[8] = (result[8] & 0x3f) | 0x80
	return result
}

func effectiveAction(row Row) Action {
	if row.Decision != "" {
		return row.Decision
	}
	return row.ProposedAction
}

func actionFromOutcome(outcome OutcomeKind) Action {
	switch outcome {
	case OutcomeInserted:
		return ActionCreate
	case OutcomeUpdated:
		return ActionUpdate
	case OutcomeLinked:
		return ActionLink
	case OutcomeSkipped:
		return ActionSkip
	default:
		return ActionError
	}
}
