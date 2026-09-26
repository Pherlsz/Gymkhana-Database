package operations

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
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
	if options.RateLimit <= 0 {
		options.RateLimit = 10
	}
	if options.MaximumActive <= 0 {
		options.MaximumActive = 3
	}
	if options.CleanupBatch <= 0 {
		options.CleanupBatch = 100
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Service{
		store:           store,
		objects:         objects,
		jobs:            jobs,
		uploadTTL:       options.UploadTTL,
		importRetention: options.ImportRetention,
		exportRetention: options.ExportRetention,
		downloadTTL:     options.DownloadTTL,
		rateLimit:       options.RateLimit,
		maximumActive:   options.MaximumActive,
		cleanupBatch:    options.CleanupBatch,
		now:             now,
		onAuditFailure:  options.OnAuditFailure,
	}, nil
}

func (service *Service) Catalog(ctx context.Context, actor auth.Session) ([]ModuleCatalog, error) {
	if !actor.User.Active || !actor.User.Role.CanUseOperations() {
		return nil, ErrForbidden
	}
	modules := SupportedModules()
	catalog := make([]ModuleCatalog, 0, len(modules))
	for _, module := range modules {
		item, err := service.catalogModule(ctx, actor.User.Role, module)
		if err != nil {
			return nil, err
		}
		catalog = append(catalog, item)
	}
	return catalog, nil
}

func (service *Service) catalogModule(ctx context.Context, role auth.Role, module Module) (ModuleCatalog, error) {
	catalog, ok := moduleCatalog(role, module)
	if !ok {
		return ModuleCatalog{}, ErrInvalidInput
	}
	customFields, err := service.store.CustomFields(ctx, module)
	if err != nil {
		return ModuleCatalog{}, err
	}
	for _, field := range customFields {
		catalog.Fields = append(catalog.Fields, field.Field)
	}
	return catalog, nil
}

func canImport(actor auth.Session, module Module) bool {
	if !actor.User.Active {
		return false
	}
	catalog, ok := moduleCatalog(actor.User.Role, module)
	return ok && catalog.CanImport
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
