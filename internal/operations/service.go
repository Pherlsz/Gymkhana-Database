package operations

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/bill"
	"github.com/Pherlsz/Gymkhana-Database/internal/customdata"
	"github.com/Pherlsz/Gymkhana-Database/internal/document"
	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
)

type AuditFailureHandler func(context.Context, AuditEvent, error)

type ServiceOptions struct {
	UploadTTL       time.Duration
	ImportRetention time.Duration
	ExportRetention time.Duration
	DownloadTTL     time.Duration
	RateLimit       int
	MaximumActive   int
	CleanupBatch    int
	Now             func() time.Time
	OnAuditFailure  AuditFailureHandler
}

type Service struct {
	store           Store
	objects         ObjectStore
	jobs            Jobs
	uploadTTL       time.Duration
	importRetention time.Duration
	exportRetention time.Duration
	downloadTTL     time.Duration
	rateLimit       int
	maximumActive   int
	cleanupBatch    int
	now             func() time.Time
	onAuditFailure  AuditFailureHandler
}

func NewService(store Store, objects ObjectStore, jobs Jobs, options ServiceOptions) (*Service, error) {
	if store == nil || objects == nil || jobs == nil {
		return nil, fmt.Errorf("configure operations: %w", ErrInvalidInput)
	}
	if options.UploadTTL == 0 {
		options.UploadTTL = 15 * time.Minute
	}
	if options.ImportRetention == 0 {
		options.ImportRetention = 24 * time.Hour
	}
	if options.ExportRetention == 0 {
		options.ExportRetention = 30 * time.Minute
	}
	if options.DownloadTTL == 0 {
		options.DownloadTTL = 5 * time.Minute
	}
	if options.RateLimit == 0 {
		options.RateLimit = 10
	}
	if options.MaximumActive == 0 {
		options.MaximumActive = 3
	}
	if options.CleanupBatch == 0 {
		options.CleanupBatch = 100
	}
	if options.UploadTTL < time.Minute || options.UploadTTL > time.Hour ||
		options.ImportRetention < time.Hour || options.ImportRetention > 7*24*time.Hour ||
		options.ExportRetention < 5*time.Minute || options.ExportRetention > 24*time.Hour ||
		options.DownloadTTL < time.Minute || options.DownloadTTL > time.Hour ||
		options.RateLimit < 1 || options.RateLimit > 1000 ||
		options.MaximumActive < 1 || options.MaximumActive > 50 ||
		options.CleanupBatch < 1 || options.CleanupBatch > 1000 {
		return nil, fmt.Errorf("configure operation limits: %w", ErrInvalidInput)
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	return &Service{
		store: store, objects: objects, jobs: jobs, uploadTTL: options.UploadTTL,
		importRetention: options.ImportRetention, exportRetention: options.ExportRetention,
		downloadTTL: options.DownloadTTL, rateLimit: options.RateLimit,
		maximumActive: options.MaximumActive, cleanupBatch: options.CleanupBatch,
		now: options.Now, onAuditFailure: options.OnAuditFailure,
	}, nil
}

func (service *Service) Catalog(ctx context.Context, actor auth.Session) ([]ModuleCatalog, error) {
	if !actor.User.Active || !actor.User.Role.CanUseOperations() {
		return nil, ErrForbidden
	}
	catalog := Catalog(actor.User.Role)
	for index := range catalog {
		fields, err := service.store.CustomFields(ctx, catalog[index].ID)
		if err != nil {
			return nil, err
		}
		for _, field := range fields {
			catalog[index].Fields = append(catalog[index].Fields, field.Field)
		}
	}
	return catalog, nil
}

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
	opened, err := service.objects.Open(ctx, value.ObjectKey)
	if err != nil {
		return Import{}, err
	}
	hash := sha256.New()
	count, copyErr := io.Copy(hash, io.LimitReader(opened, MaximumFileSize+1))
	closeErr := opened.Close()
	if copyErr != nil || closeErr != nil {
		return Import{}, fmt.Errorf("verify import upload: %w", errors.Join(copyErr, closeErr))
	}
	if count != value.DeclaredSize || count > MaximumFileSize {
		return Import{}, ErrWorkbookLimit
	}
	var digest [32]byte
	copy(digest[:], hash.Sum(nil))
	jobID, err := service.jobs.EnqueueParse(ctx, id)
	if err != nil {
		return Import{}, err
	}
	now := service.now().UTC()
	confirmed, err := service.store.ConfirmImport(ctx, id, actor.User.ID, count, digest, jobID, now)
	if err != nil {
		// A concurrent confirmation may have received the same River unique job.
		// Leaving it in place is safe: workers retry until the state transition is visible.
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
	if digest != value.ContentSHA256 {
		return service.failImport(ctx, value, ImportParsing, "content_changed", workerRequestID("parse", id), ErrConflict)
	}
	now := service.now().UTC()
	if err := service.store.StageWorkbook(ctx, id, workbook, now); err != nil {
		if errors.Is(err, ErrCancelled) || errors.Is(err, ErrInvalidState) {
			return err
		}
		return service.failImport(ctx, value, ImportParsing, workbookErrorCode(err), workerRequestID("parse", id), err)
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
	selected, err := service.store.SelectSheet(ctx, id, actor.User.ID, version, sheetIndex, service.now().UTC())
	if err == nil {
		service.audit(ctx, &actor.User.ID, &id, nil, value.Module, AuditImportMapped, auth.AuditOutcomeSuccess, nil, requestID)
	}
	return selected, err
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

// ValidateMapping applies the same allowlisted logical-field constraints to
// interactive XLSX mappings and provider-backed staging sources.
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
	customFields, err := service.store.CustomFields(ctx, value.Module)
	if err != nil {
		return Import{}, err
	}
	rows, err := service.store.LoadMappedRows(ctx, id)
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
			targetID, _, action, targetErr := targetFromValues(mapped.Values)
			if targetErr != nil {
				row.ProposedAction = ActionError
				row.ValidationErrorCount++
				markFirstError(&row, "invalid_target")
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
	saved, err := service.store.SavePreview(ctx, id, actor.User.ID, version, preview, unresolved, validationErrors, service.now().UTC())
	if err == nil {
		service.audit(ctx, &actor.User.ID, &id, nil, value.Module, AuditImportPreviewed, auth.AuditOutcomeSuccess, nil, requestID)
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
		// A concurrent execution request can share the same River unique job.
		// Do not cancel work that may belong to the successful request.
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

func (service *Service) CreateExport(ctx context.Context, actor auth.Session, module Module, idempotencyKey, requestID string) (Export, error) {
	catalog, ok := moduleCatalog(actor.User.Role, module)
	if !actor.User.Active || !ok || !catalog.CanExport {
		service.audit(ctx, &actor.User.ID, nil, nil, module, AuditExportCreated, auth.AuditOutcomeDenied, nil, requestID)
		return Export{}, ErrForbidden
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if !validIdempotencyKey(idempotencyKey) {
		return Export{}, ErrInvalidInput
	}
	id, err := NewIdentifier()
	if err != nil {
		return Export{}, err
	}
	now := service.now().UTC()
	jobID, err := service.jobs.EnqueueExport(ctx, id)
	if err != nil {
		return Export{}, err
	}
	value := Export{
		ID: id, ActorUserID: actor.User.ID, Module: module, IdempotencyKey: idempotencyKey,
		State: ExportQueued, ObjectKey: exportObjectKey(id, now), Filename: exportFilename(module, now),
		RiverJobID: jobID, ExpiresAt: now.Add(service.exportRetention), CreatedAt: now, UpdatedAt: now,
	}
	created, err := service.store.CreateExport(ctx, value, service.limits(now))
	if err != nil {
		_ = service.jobs.Cancel(ctx, jobID)
		return Export{}, err
	}
	if created.ID != id {
		// Export IDs are part of the unique River arguments. This job can only
		// target the discarded candidate, so it is safe to cancel on replay.
		_ = service.jobs.Cancel(ctx, jobID)
		if created.Module != module || created.State == ExportFailed || created.State == ExportCancelled || created.State == ExportExpired || !created.ExpiresAt.After(now) {
			service.audit(ctx, &actor.User.ID, nil, &created.ID, created.Module, AuditExportCreated, auth.AuditOutcomeFailure, nil, requestID)
			return Export{}, ErrConflict
		}
	}
	service.audit(ctx, &actor.User.ID, nil, &created.ID, module, AuditExportCreated, auth.AuditOutcomeSuccess, nil, requestID)
	return created, nil
}

func (service *Service) GenerateExport(ctx context.Context, id Identifier) error {
	value, err := service.store.BeginExport(ctx, id, service.now().UTC())
	if err != nil {
		return err
	}
	if value.State == ExportCompleted {
		return nil
	}
	actor, err := service.store.GetActor(ctx, value.ActorUserID)
	catalog, ok := moduleCatalog(actor.User.Role, value.Module)
	if err != nil || !actor.User.Active || !ok || !catalog.CanExport {
		return service.failExport(ctx, value, ExportRunning, "actor_forbidden", workerRequestID("export", id), ErrForbidden)
	}
	dataset, err := service.store.ExportDataset(ctx, value.Module)
	if err != nil {
		return service.failExport(ctx, value, ExportRunning, "query_failed", workerRequestID("export", id), err)
	}
	data, err := WriteWorkbook(value.Module.Label(), dataset.Headers, dataset.Rows)
	if err != nil {
		return service.failExport(ctx, value, ExportRunning, "generation_failed", workerRequestID("export", id), err)
	}
	if err := service.objects.Put(ctx, value.ObjectKey, bytes.NewReader(data), int64(len(data)), "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"); err != nil {
		return service.failExport(ctx, value, ExportRunning, "storage_unavailable", workerRequestID("export", id), err)
	}
	digest := sha256.Sum256(data)
	completed, err := service.store.CompleteExport(ctx, id, len(dataset.Rows), int64(len(data)), digest, service.now().UTC())
	if err != nil {
		return err
	}
	service.audit(ctx, &value.ActorUserID, nil, &id, value.Module, AuditExportCompleted, auth.AuditOutcomeSuccess, &completed.RowCount, workerRequestID("export", id))
	return nil
}

func (service *Service) GetExport(ctx context.Context, actor auth.Session, id Identifier) (Export, error) {
	if !actor.User.Active || !actor.User.Role.CanUseOperations() {
		return Export{}, ErrForbidden
	}
	return service.store.GetExport(ctx, id, actor.User.ID)
}

func (service *Service) ListExports(ctx context.Context, actor auth.Session, options ListOptions) (ExportPage, error) {
	if !actor.User.Active || !actor.User.Role.CanUseOperations() {
		return ExportPage{}, ErrForbidden
	}
	return service.store.ListExports(ctx, actor.User.ID, options)
}

func (service *Service) DownloadExport(ctx context.Context, actor auth.Session, id Identifier, requestID string) (DownloadGrant, error) {
	value, err := service.store.GetExport(ctx, id, actor.User.ID)
	if err != nil {
		return DownloadGrant{}, err
	}
	catalog, ok := moduleCatalog(actor.User.Role, value.Module)
	if !actor.User.Active || !ok || !catalog.CanExport {
		return DownloadGrant{}, ErrForbidden
	}
	if value.State != ExportCompleted || !value.ExpiresAt.After(service.now().UTC()) {
		return DownloadGrant{}, ErrInvalidState
	}
	signed, err := service.objects.PresignDownload(ctx, value.ObjectKey, value.Filename, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", service.downloadTTL)
	if err != nil {
		return DownloadGrant{}, err
	}
	service.audit(ctx, &actor.User.ID, nil, &id, value.Module, AuditExportDownloaded, auth.AuditOutcomeSuccess, nil, requestID)
	return DownloadGrant{URL: signed.URL, Method: signed.Method, ExpiresAt: signed.ExpiresAt}, nil
}

func (service *Service) BulkDelete(ctx context.Context, actor auth.Session, module Module, items []BulkItem, confirmation, requestID string) (BulkDeleteResult, error) {
	catalog, ok := moduleCatalog(actor.User.Role, module)
	if !actor.User.Active || !ok || !catalog.CanBulkDelete {
		service.audit(ctx, &actor.User.ID, nil, nil, module, AuditBulkDelete, auth.AuditOutcomeDenied, nil, requestID)
		return BulkDeleteResult{}, ErrForbidden
	}
	if confirmation != BulkDeleteConfirmation {
		return BulkDeleteResult{}, ErrInvalidConfirmation
	}
	if len(items) == 0 || len(items) > MaximumBulkSelection {
		return BulkDeleteResult{}, ErrInvalidInput
	}
	deleted, err := service.store.BulkDelete(ctx, actor.User.ID, module, items, requestID, service.now().UTC())
	if err != nil {
		service.audit(ctx, &actor.User.ID, nil, nil, module, AuditBulkDelete, auth.AuditOutcomeFailure, nil, requestID)
		return BulkDeleteResult{}, err
	}
	return BulkDeleteResult{Module: module, Deleted: deleted}, nil
}

func (service *Service) Cleanup(ctx context.Context) (int, error) {
	now := service.now().UTC()
	candidates, err := service.store.ClaimCleanupCandidates(ctx, now, service.cleanupBatch)
	if err != nil {
		return 0, err
	}
	cleaned := 0
	for _, candidate := range candidates {
		if candidate.RequiresObjectDeletion {
			if err := service.objects.Delete(ctx, candidate.ObjectKey); err != nil {
				releaseErr := service.store.ReleaseCleanupCandidate(ctx, candidate)
				return cleaned, errors.Join(err, releaseErr)
			}
		}
		if err := service.store.MarkObjectDeleted(ctx, candidate, now); err != nil {
			return cleaned, err
		}
		if candidate.Kind == "IMPORT" {
			service.audit(ctx, &candidate.ActorUserID, &candidate.ID, nil, candidate.Module, AuditImportExpired, auth.AuditOutcomeSuccess, nil, workerRequestID("cleanup", candidate.ID))
		} else {
			service.audit(ctx, &candidate.ActorUserID, nil, &candidate.ID, candidate.Module, AuditExportExpired, auth.AuditOutcomeSuccess, nil, workerRequestID("cleanup", candidate.ID))
		}
		cleaned++
	}
	return cleaned, nil
}

func (service *Service) ScheduleCleanup(ctx context.Context) error {
	window := service.now().UTC().Truncate(15 * time.Minute)
	_, err := service.jobs.EnqueueCleanup(ctx, window)
	return err
}

func (service *Service) failImport(ctx context.Context, value Import, expected ImportState, code, requestID string, cause error) error {
	changed, failErr := service.store.FailImport(ctx, value.ID, expected, code, service.now().UTC())
	if changed {
		service.audit(ctx, &value.ActorUserID, &value.ID, nil, value.Module, AuditImportFailed, auth.AuditOutcomeFailure, nil, requestID)
	}
	return errors.Join(cause, failErr)
}

func (service *Service) failExport(ctx context.Context, value Export, expected ExportState, code, requestID string, cause error) error {
	changed, failErr := service.store.FailExport(ctx, value.ID, expected, code, service.now().UTC())
	if changed {
		service.audit(ctx, &value.ActorUserID, nil, &value.ID, value.Module, AuditExportFailed, auth.AuditOutcomeFailure, nil, requestID)
	}
	return errors.Join(cause, failErr)
}

func (service *Service) catalogModule(ctx context.Context, role auth.Role, module Module) (ModuleCatalog, error) {
	catalog, ok := moduleCatalog(role, module)
	if !ok {
		return ModuleCatalog{}, ErrForbidden
	}
	fields, err := service.store.CustomFields(ctx, module)
	if err != nil {
		return ModuleCatalog{}, err
	}
	for _, field := range fields {
		catalog.Fields = append(catalog.Fields, field.Field)
	}
	return catalog, nil
}

func (service *Service) mutationForRow(ctx context.Context, module Module, values map[string]string, customFields []CustomFieldDefinition) (Mutation, int64, error) {
	effective := make(map[string]string, len(values))
	for field, value := range values {
		effective[field] = value
	}
	targetID, _, action, err := targetFromValues(values)
	if err != nil {
		return Mutation{}, 0, err
	}
	var canonicalVersion int64
	if action == ActionUpdate {
		current, err := service.store.CanonicalValues(ctx, module, *targetID)
		if err != nil {
			return Mutation{}, 0, err
		}
		canonicalVersion, err = strconv.ParseInt(current["version"], 10, 64)
		if err != nil || canonicalVersion <= 0 {
			return Mutation{}, 0, ErrInvalidState
		}
		for field, value := range current {
			if _, mapped := effective[field]; !mapped {
				effective[field] = value
			}
		}
	}
	custom, err := normalizeCustomValues(effective, customFields)
	if err != nil {
		return Mutation{}, 0, err
	}
	switch module {
	case ModuleProfiles:
		normalized, err := profile.Normalize(profile.Values{
			FullName: effective["full_name"], SocialName: effective["social_name"], CPF: effective["cpf"],
			Email: effective["email"], MobilePhone: effective["mobile_phone"], LandlinePhone: effective["landline_phone"],
			Address: profile.Address{Street: effective["address_street"], Number: effective["address_number"],
				Complement: effective["address_complement"], Neighborhood: effective["address_neighborhood"],
				City: effective["address_city"], State: effective["address_state"], PostalCode: effective["address_postal_code"]},
			Notes: effective["notes"],
		})
		if err != nil {
			return Mutation{}, 0, err
		}
		return Mutation{Module: module, Profile: &normalized, CustomValues: custom}, canonicalVersion, nil
	case ModuleDocuments:
		ownerID, err := profile.ParseIdentifier(effective["owner_profile_id"])
		if err != nil {
			return Mutation{}, 0, err
		}
		typeID, err := document.ParseIdentifier(effective["document_type_id"])
		if err != nil {
			return Mutation{}, 0, err
		}
		definition, err := service.store.DocumentType(ctx, typeID)
		if err != nil {
			return Mutation{}, 0, err
		}
		normalized, err := document.Normalize(document.Values{OwnerProfileID: ownerID, TypeID: typeID,
			Identifier: effective["identifier_value"], DocumentDate: effective["document_date"],
			Notes: effective["notes"], RecordState: document.RecordState(effective["record_state"])}, definition)
		if err != nil {
			return Mutation{}, 0, err
		}
		return Mutation{Module: module, Document: &normalized, CustomValues: custom}, canonicalVersion, nil
	case ModuleBills:
		ownerID, err := profile.ParseIdentifier(effective["owner_profile_id"])
		if err != nil {
			return Mutation{}, 0, err
		}
		typeID, err := bill.ParseIdentifier(effective["bill_type_id"])
		if err != nil {
			return Mutation{}, 0, err
		}
		definition, err := service.store.BillType(ctx, typeID)
		if err != nil {
			return Mutation{}, 0, err
		}
		normalized, err := bill.Normalize(bill.Values{OwnerProfileID: ownerID, TypeID: typeID,
			PrintedHolderName: effective["printed_holder_name"], PrintedAddress: effective["printed_address"],
			Reference: effective["reference_value"], Competence: effective["competence"], Amount: effective["amount"],
			Currency: effective["currency"], Notes: effective["notes"], RecordState: bill.RecordState(effective["record_state"])}, definition)
		if err != nil {
			return Mutation{}, 0, err
		}
		return Mutation{Module: module, Bill: &normalized, CustomValues: custom}, canonicalVersion, nil
	default:
		return Mutation{}, 0, ErrInvalidInput
	}
}

func normalizeCustomValues(values map[string]string, fields []CustomFieldDefinition) ([]CustomValueMutation, error) {
	allowed := make(map[string]CustomFieldDefinition, len(fields))
	for _, field := range fields {
		allowed[field.Field.ID] = field
	}
	for key := range values {
		if strings.HasPrefix(key, CustomFieldPrefix) {
			if _, ok := allowed[key]; !ok {
				return nil, ErrInvalidMapping
			}
		}
	}
	result := make([]CustomValueMutation, 0, len(fields))
	for _, field := range fields {
		raw, mapped := values[field.Field.ID]
		if !mapped {
			continue
		}
		input, err := customValueInput(field.Definition, raw)
		if err != nil {
			return nil, err
		}
		result = append(result, CustomValueMutation{Definition: field.Definition, Value: input})
	}
	return result, nil
}

func customValueInput(definition customdata.FieldDefinition, raw string) (customdata.ValueInput, error) {
	value := strings.TrimSpace(raw)
	input := customdata.ValueInput{FieldDefinitionID: definition.ID, Kind: definition.Values.Kind}
	switch definition.Values.Kind {
	case customdata.FieldText, customdata.FieldLongText, customdata.FieldEmail, customdata.FieldPhone:
		input.Text = value
	case customdata.FieldInteger:
		if value != "" {
			parsed, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return customdata.ValueInput{}, ErrInvalidInput
			}
			input.Integer = &parsed
		}
	case customdata.FieldDecimal:
		input.Decimal = value
	case customdata.FieldBoolean:
		if value != "" {
			parsed, err := strconv.ParseBool(strings.ToLower(value))
			if err != nil {
				return customdata.ValueInput{}, ErrInvalidInput
			}
			input.Boolean = &parsed
		}
	case customdata.FieldCivilDate:
		input.CivilDate = value
	case customdata.FieldCivilMonth:
		input.CivilMonth = value
	default:
		return customdata.ValueInput{}, ErrInvalidMapping
	}
	normalized, err := customdata.NormalizeValue(input, definition)
	if err != nil {
		return customdata.ValueInput{}, err
	}
	return normalized, nil
}

func targetFromValues(values map[string]string) (*Identifier, int64, Action, error) {
	rawID := strings.TrimSpace(values["record_id"])
	rawVersion := strings.TrimSpace(values["version"])
	if rawID == "" && rawVersion == "" {
		return nil, 0, ActionCreate, nil
	}
	if rawID == "" || rawVersion == "" {
		return nil, 0, "", ErrInvalidInput
	}
	id, err := ParseIdentifier(rawID)
	if err != nil {
		return nil, 0, "", err
	}
	version, err := strconv.ParseInt(rawVersion, 10, 64)
	if err != nil || version <= 0 {
		return nil, 0, "", ErrInvalidInput
	}
	return &id, version, ActionUpdate, nil
}

func canImport(actor auth.Session, module Module) bool {
	if !actor.User.Active {
		return false
	}
	catalog, ok := moduleCatalog(actor.User.Role, module)
	return ok && catalog.CanImport
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

func sanitizeFilename(value string) string {
	value = filepath.Base(strings.TrimSpace(strings.ToValidUTF8(value, "")))
	value = strings.Map(func(character rune) rune {
		if unicode.IsControl(character) || character == '/' || character == '\\' {
			return -1
		}
		return character
	}, value)
	return strings.TrimSpace(value)
}

func validXLSXFilename(value string) bool {
	return value != "" && utf8.RuneCountInString(value) <= 255 && strings.EqualFold(filepath.Ext(value), ".xlsx")
}

func validIdempotencyKey(value string) bool {
	if len(value) < 8 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) || unicode.IsSpace(character) {
			return false
		}
	}
	return true
}

func importObjectKey(id Identifier, now time.Time) string {
	return fmt.Sprintf("operations/imports/%04d/%02d/%s.xlsx", now.Year(), now.Month(), id.String())
}

func exportObjectKey(id Identifier, now time.Time) string {
	return fmt.Sprintf("operations/exports/%04d/%02d/%s.xlsx", now.Year(), now.Month(), id.String())
}

func exportFilename(module Module, now time.Time) string {
	return strings.ToLower(string(module)) + "-" + now.Format("20060102-150405") + ".xlsx"
}

func (service *Service) limits(now time.Time) Limits {
	return Limits{WindowStart: now.UTC().Truncate(time.Minute), MaximumRequests: service.rateLimit, MaximumActive: service.maximumActive}
}

func workbookErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrWorkbookLimit):
		return "workbook_limit"
	case errors.Is(err, ErrUnsupportedWorkbook):
		return "unsupported_workbook"
	default:
		return "parse_failed"
	}
}

func workerRequestID(kind string, id Identifier) string { return "worker-" + kind + "-" + id.String() }

func (service *Service) audit(ctx context.Context, actorID *auth.Identifier, importID, exportID *Identifier, module Module, eventType AuditEventType, outcome auth.AuditOutcome, affected *int, requestID string) {
	id, err := NewIdentifier()
	if err != nil {
		service.reportAuditFailure(ctx, AuditEvent{EventType: eventType}, err)
		return
	}
	event := AuditEvent{ID: id, ActorUserID: actorID, ImportID: importID, ExportID: exportID, Module: module,
		EventType: eventType, Outcome: outcome, AffectedCount: affected, RequestID: requestID, CreatedAt: service.now().UTC()}
	if err := service.store.RecordAuditEvent(ctx, event); err != nil {
		service.reportAuditFailure(ctx, event, err)
	}
}

func (service *Service) reportAuditFailure(ctx context.Context, event AuditEvent, err error) {
	if service.onAuditFailure != nil {
		service.onAuditFailure(ctx, event, err)
	}
}
