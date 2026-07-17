package attachment

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

const DeleteConfirmation = "Confirmar"

type AuditFailureHandler func(context.Context, AuditEvent, error)

type ServiceOptions struct {
	UploadTTL         time.Duration
	DownloadTTL       time.Duration
	TrashRetention    time.Duration
	MaximumFileSize   int64
	MaximumTotalBytes int64
	UploadRateLimit   int
	CleanupBatch      int
	Now               func() time.Time
	OnAuditFailure    AuditFailureHandler
}

type UploadGrant struct {
	Intent UploadIntent
	Upload SignedRequest
}

type CleanupResult struct {
	ExpiredUploads int
	Purged         int
	Failures       int
	Skipped        bool
}

type Service struct {
	store             Store
	audit             AuditStore
	objects           ObjectStore
	uploadTTL         time.Duration
	downloadTTL       time.Duration
	trashRetention    time.Duration
	maximumFileSize   int64
	maximumTotalBytes int64
	uploadRateLimit   int
	cleanupBatch      int
	now               func() time.Time
	onAuditFailure    AuditFailureHandler
}

func NewService(store Store, objects ObjectStore, options ServiceOptions) (*Service, error) {
	if store == nil || objects == nil {
		return nil, ErrInvalidServiceSetup
	}
	audit, ok := store.(AuditStore)
	if !ok {
		return nil, fmt.Errorf("%w: audit store is unavailable", ErrInvalidServiceSetup)
	}
	if options.UploadTTL <= 0 || options.UploadTTL > time.Hour {
		return nil, fmt.Errorf("%w: upload TTL must be between zero and one hour", ErrInvalidServiceSetup)
	}
	if options.DownloadTTL <= 0 || options.DownloadTTL > time.Hour {
		return nil, fmt.Errorf("%w: download TTL must be between zero and one hour", ErrInvalidServiceSetup)
	}
	if options.TrashRetention < 24*time.Hour {
		return nil, fmt.Errorf("%w: trash retention must be at least one day", ErrInvalidServiceSetup)
	}
	if options.MaximumFileSize <= 0 {
		return nil, fmt.Errorf("%w: maximum file size must be positive", ErrInvalidServiceSetup)
	}
	if options.MaximumTotalBytes == 0 {
		options.MaximumTotalBytes = 5 << 30
	}
	if options.MaximumTotalBytes < options.MaximumFileSize {
		return nil, fmt.Errorf("%w: total storage quota must cover at least one maximum-sized file", ErrInvalidServiceSetup)
	}
	if options.UploadRateLimit == 0 {
		options.UploadRateLimit = 12
	}
	if options.UploadRateLimit < 1 || options.UploadRateLimit > 1000 {
		return nil, fmt.Errorf("%w: upload rate limit must be between 1 and 1000", ErrInvalidServiceSetup)
	}
	if options.CleanupBatch <= 0 {
		options.CleanupBatch = 100
	}
	if options.CleanupBatch > 1000 {
		return nil, fmt.Errorf("%w: cleanup batch cannot exceed 1000", ErrInvalidServiceSetup)
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	return &Service{
		store:             store,
		audit:             audit,
		objects:           objects,
		uploadTTL:         options.UploadTTL,
		downloadTTL:       options.DownloadTTL,
		trashRetention:    options.TrashRetention,
		maximumFileSize:   options.MaximumFileSize,
		maximumTotalBytes: options.MaximumTotalBytes,
		uploadRateLimit:   options.UploadRateLimit,
		cleanupBatch:      options.CleanupBatch,
		now:               options.Now,
		onAuditFailure:    options.OnAuditFailure,
	}, nil
}

func (service *Service) CreateUploadIntent(ctx context.Context, actor auth.Session, input CreateUploadIntentInput, requestID string) (UploadGrant, error) {
	intentID, err := NewIdentifier()
	if err != nil {
		return UploadGrant{}, fmt.Errorf("generate attachment upload intent identifier: %w", err)
	}
	if !actor.User.Active || !canWriteOwner(actor.User.Role, input.Owner) {
		service.recordAudit(ctx, &actor.User.ID, nil, &intentID, &input.Owner, AuditUploadIntentCreated, auth.AuditOutcomeDenied, requestID)
		return UploadGrant{}, ErrForbidden
	}
	normalized, err := service.normalizeUploadInput(input)
	if err != nil {
		service.recordAudit(ctx, &actor.User.ID, nil, &intentID, &input.Owner, AuditUploadIntentCreated, auth.AuditOutcomeFailure, requestID)
		return UploadGrant{}, err
	}
	now := service.now().UTC()
	intent := UploadIntent{
		ID:               intentID,
		ActorUserID:      actor.User.ID,
		Owner:            normalized.Owner,
		OriginalFileName: normalized.OriginalFileName,
		DeclaredMIME:     normalized.DeclaredMIME,
		ExpectedSize:     normalized.ExpectedSize,
		ObjectKey:        objectKey(intentID, now),
		ExpiresAt:        now.Add(service.uploadTTL),
		CreatedAt:        now,
	}
	created, err := service.store.CreateUploadIntent(ctx, intent, UploadLimits{
		RateWindowStart:   now.Add(-time.Minute),
		MaximumIntents:    service.uploadRateLimit,
		MaximumTotalBytes: service.maximumTotalBytes,
	})
	if err != nil {
		service.recordAudit(ctx, &actor.User.ID, nil, &intentID, &normalized.Owner, AuditUploadIntentCreated, mutationOutcome(err), requestID)
		return UploadGrant{}, err
	}
	signed, err := service.objects.PresignUpload(ctx, created.ObjectKey, service.uploadTTL)
	if err != nil {
		service.recordAudit(ctx, &actor.User.ID, nil, &intentID, &normalized.Owner, AuditUploadIntentCreated, auth.AuditOutcomeFailure, requestID)
		return UploadGrant{}, err
	}
	service.recordAudit(ctx, &actor.User.ID, nil, &intentID, &normalized.Owner, AuditUploadIntentCreated, auth.AuditOutcomeSuccess, requestID)
	return UploadGrant{Intent: created, Upload: signed}, nil
}

func (service *Service) ConfirmUpload(ctx context.Context, actor auth.Session, intentID Identifier, requestID string) (Attachment, error) {
	intent, err := service.store.GetUploadIntent(ctx, intentID)
	if err != nil {
		service.recordAudit(ctx, &actor.User.ID, nil, &intentID, nil, AuditUploadConfirmed, mutationOutcome(err), requestID)
		return Attachment{}, err
	}
	if !actor.User.Active || actor.User.ID != intent.ActorUserID || !canWriteOwner(actor.User.Role, intent.Owner) {
		service.recordAudit(ctx, &actor.User.ID, nil, &intentID, &intent.Owner, AuditUploadConfirmed, auth.AuditOutcomeDenied, requestID)
		return Attachment{}, ErrForbidden
	}
	now := service.now().UTC()
	if intent.ConsumedAt != nil {
		return Attachment{}, ErrUploadIntentConsumed
	}
	if !intent.ExpiresAt.After(now) {
		return Attachment{}, ErrUploadIntentExpired
	}
	reader, err := service.objects.Open(ctx, intent.ObjectKey)
	if err != nil {
		service.recordAudit(ctx, &actor.User.ID, nil, &intentID, &intent.Owner, AuditUploadConfirmed, mutationOutcome(err), requestID)
		return Attachment{}, err
	}
	verified, verifyErr := VerifyObject(reader, intent.DeclaredMIME, intent.ExpectedSize, service.maximumFileSize)
	closeErr := reader.Close()
	if verifyErr != nil {
		_ = service.objects.Delete(ctx, intent.ObjectKey)
		service.recordAudit(ctx, &actor.User.ID, nil, &intentID, &intent.Owner, AuditUploadConfirmed, auth.AuditOutcomeFailure, requestID)
		return Attachment{}, verifyErr
	}
	if closeErr != nil {
		return Attachment{}, fmt.Errorf("close attachment object: %w", closeErr)
	}
	attachmentID, err := NewIdentifier()
	if err != nil {
		return Attachment{}, fmt.Errorf("generate attachment identifier: %w", err)
	}
	created, err := service.store.ConfirmUploadIntent(ctx, intentID, actor.User.ID, attachmentID, verified, now)
	if err != nil {
		service.recordAudit(ctx, &actor.User.ID, &attachmentID, &intentID, &intent.Owner, AuditUploadConfirmed, mutationOutcome(err), requestID)
		return Attachment{}, err
	}
	service.recordAudit(ctx, &actor.User.ID, &created.ID, &intentID, &created.Owner, AuditUploadConfirmed, auth.AuditOutcomeSuccess, requestID)
	return created, nil
}

func (service *Service) List(ctx context.Context, actor auth.Session, owner OwnerReference, includeTrashed bool) ([]Attachment, error) {
	if !actor.User.Active || !canReadOwner(actor.User.Role, owner) {
		return nil, ErrForbidden
	}
	if !owner.Valid() {
		return nil, ErrInvalidOwner
	}
	return service.store.List(ctx, owner, includeTrashed)
}

func (service *Service) Download(ctx context.Context, actor auth.Session, id Identifier, requestID string) (SignedRequest, error) {
	value, err := service.store.Get(ctx, id)
	if err != nil {
		return SignedRequest{}, err
	}
	if !actor.User.Active || !canReadOwner(actor.User.Role, value.Owner) {
		service.recordAudit(ctx, &actor.User.ID, &id, nil, &value.Owner, AuditDownloadRequested, auth.AuditOutcomeDenied, requestID)
		return SignedRequest{}, ErrForbidden
	}
	if value.LifecycleState != LifecycleActive {
		return SignedRequest{}, ErrInvalidState
	}
	signed, err := service.objects.PresignDownload(ctx, value.ObjectKey, value.OriginalFileName, value.DetectedMIME, service.downloadTTL)
	if err != nil {
		service.recordAudit(ctx, &actor.User.ID, &id, nil, &value.Owner, AuditDownloadRequested, auth.AuditOutcomeFailure, requestID)
		return SignedRequest{}, err
	}
	service.recordAudit(ctx, &actor.User.ID, &id, nil, &value.Owner, AuditDownloadRequested, auth.AuditOutcomeSuccess, requestID)
	return signed, nil
}

func (service *Service) Trash(ctx context.Context, actor auth.Session, id Identifier, version int64, confirmation, requestID string) (Attachment, error) {
	value, err := service.store.Get(ctx, id)
	if err != nil {
		return Attachment{}, err
	}
	if !actor.User.Active || !canWriteOwner(actor.User.Role, value.Owner) {
		service.recordAudit(ctx, &actor.User.ID, &id, nil, &value.Owner, AuditTrashed, auth.AuditOutcomeDenied, requestID)
		return Attachment{}, ErrForbidden
	}
	if confirmation != DeleteConfirmation {
		return Attachment{}, ErrInvalidConfirmation
	}
	now := service.now().UTC()
	updated, err := service.store.Trash(ctx, id, version, now, now.Add(service.trashRetention))
	if err != nil {
		service.recordAudit(ctx, &actor.User.ID, &id, nil, &value.Owner, AuditTrashed, mutationOutcome(err), requestID)
		return Attachment{}, err
	}
	service.recordAudit(ctx, &actor.User.ID, &id, nil, &updated.Owner, AuditTrashed, auth.AuditOutcomeSuccess, requestID)
	return updated, nil
}

func (service *Service) Restore(ctx context.Context, actor auth.Session, id Identifier, version int64, requestID string) (Attachment, error) {
	value, err := service.store.Get(ctx, id)
	if err != nil {
		return Attachment{}, err
	}
	if !actor.User.Active || !canWriteOwner(actor.User.Role, value.Owner) {
		service.recordAudit(ctx, &actor.User.ID, &id, nil, &value.Owner, AuditRestored, auth.AuditOutcomeDenied, requestID)
		return Attachment{}, ErrForbidden
	}
	updated, err := service.store.Restore(ctx, id, version, service.now().UTC())
	if err != nil {
		service.recordAudit(ctx, &actor.User.ID, &id, nil, &value.Owner, AuditRestored, mutationOutcome(err), requestID)
		return Attachment{}, err
	}
	service.recordAudit(ctx, &actor.User.ID, &id, nil, &updated.Owner, AuditRestored, auth.AuditOutcomeSuccess, requestID)
	return updated, nil
}

func (service *Service) Cleanup(ctx context.Context, requestID string) (result CleanupResult, err error) {
	lease, acquired, err := service.store.AcquireCleanupLease(ctx)
	if err != nil {
		return result, err
	}
	if !acquired {
		result.Skipped = true
		return result, nil
	}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if releaseErr := lease.Release(releaseCtx); releaseErr != nil && err == nil {
			err = releaseErr
		}
	}()

	now := service.now().UTC()
	expired, err := service.store.ListExpiredUploadIntents(ctx, now, service.cleanupBatch)
	if err != nil {
		return result, err
	}
	for _, intent := range expired {
		if err := service.objects.Delete(ctx, intent.ObjectKey); err != nil {
			result.Failures++
			service.recordAudit(ctx, nil, nil, &intent.ID, &intent.Owner, AuditExpiredUploadCleaned, auth.AuditOutcomeFailure, requestID)
			continue
		}
		if err := service.store.DeleteExpiredUploadIntent(ctx, intent.ID, now); err != nil {
			result.Failures++
			service.recordAudit(ctx, nil, nil, &intent.ID, &intent.Owner, AuditExpiredUploadCleaned, auth.AuditOutcomeFailure, requestID)
			continue
		}
		result.ExpiredUploads++
		service.recordAudit(ctx, nil, nil, &intent.ID, &intent.Owner, AuditExpiredUploadCleaned, auth.AuditOutcomeSuccess, requestID)
	}
	purgeDue, err := service.store.ListPurgeDue(ctx, now, service.cleanupBatch)
	if err != nil {
		return result, err
	}
	for _, value := range purgeDue {
		if err := service.objects.Delete(ctx, value.ObjectKey); err != nil {
			result.Failures++
			service.recordAudit(ctx, nil, &value.ID, nil, &value.Owner, AuditPurged, auth.AuditOutcomeFailure, requestID)
			continue
		}
		if err := service.store.DeletePurged(ctx, value.ID, value.Version); err != nil {
			result.Failures++
			service.recordAudit(ctx, nil, &value.ID, nil, &value.Owner, AuditPurged, auth.AuditOutcomeFailure, requestID)
			continue
		}
		result.Purged++
		service.recordAudit(ctx, nil, &value.ID, nil, &value.Owner, AuditPurged, auth.AuditOutcomeSuccess, requestID)
	}
	return result, nil
}
func (service *Service) normalizeUploadInput(input CreateUploadIntentInput) (CreateUploadIntentInput, error) {
	validation := &ValidationError{}
	if !input.Owner.Valid() {
		validation.add("owner", "invalid_value")
	}
	fileName := strings.TrimSpace(strings.ToValidUTF8(filepath.Base(strings.ReplaceAll(input.OriginalFileName, "\\", "/")), ""))
	if fileName == "" || fileName == "." || fileName == ".." {
		validation.add("original_filename", "required")
	} else if len([]rune(fileName)) > MaxFileNameLength {
		validation.add("original_filename", "too_long")
	}
	mime := normalizeMIME(input.DeclaredMIME)
	if mime == "" || len(mime) > MaxMIMELength || !supportedDeclaredMIME(mime) {
		validation.add("declared_mime", "unsupported")
	}
	if input.ExpectedSize <= 0 {
		validation.add("expected_size", "invalid_value")
	} else if input.ExpectedSize > service.maximumFileSize {
		validation.add("expected_size", "too_large")
	}
	if len(validation.Fields) > 0 {
		return CreateUploadIntentInput{}, validation
	}
	input.OriginalFileName = fileName
	input.DeclaredMIME = mime
	return input, nil
}

func supportedDeclaredMIME(value string) bool {
	for _, detected := range []string{
		"application/pdf", "image/jpeg", "image/png", "image/webp", "image/gif",
		"text/plain", "text/csv",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"audio/mpeg", "audio/wav", "video/mp4", "video/webm",
	} {
		if mimeCompatible(value, detected) {
			return true
		}
	}
	return false
}

func objectKey(id Identifier, now time.Time) string {
	return fmt.Sprintf("attachments/%04d/%02d/%s", now.Year(), now.Month(), id.String())
}

func canReadOwner(role auth.Role, owner OwnerReference) bool {
	switch owner.Kind {
	case OwnerDocument:
		return role.CanReadDocuments()
	case OwnerBill:
		return role.CanReadBills()
	case OwnerCustomField:
		switch owner.CustomTargetKind {
		case CustomTargetProfile:
			return role.CanReadProfiles()
		case CustomTargetDocument:
			return role.CanReadDocuments()
		case CustomTargetBill:
			return role.CanReadBills()
		case CustomTargetCustomEntity:
			return role.Valid()
		}
	}
	return false
}

func canWriteOwner(role auth.Role, owner OwnerReference) bool {
	switch owner.Kind {
	case OwnerDocument:
		return role.CanWriteDocuments()
	case OwnerBill:
		return role.CanWriteBills()
	case OwnerCustomField:
		switch owner.CustomTargetKind {
		case CustomTargetProfile:
			return role.CanWriteProfiles()
		case CustomTargetDocument:
			return role.CanWriteDocuments()
		case CustomTargetBill:
			return role.CanWriteBills()
		case CustomTargetCustomEntity:
			return role.Valid()
		}
	}
	return false
}

func mutationOutcome(err error) auth.AuditOutcome {
	if errors.Is(err, ErrForbidden) || errors.Is(err, ErrInvalidConfirmation) {
		return auth.AuditOutcomeDenied
	}
	return auth.AuditOutcomeFailure
}

func (service *Service) recordAudit(ctx context.Context, actor *auth.Identifier, attachmentID, intentID *Identifier, owner *OwnerReference, eventType AuditEventType, outcome auth.AuditOutcome, requestID string) {
	id, err := NewIdentifier()
	if err != nil {
		return
	}
	event := AuditEvent{ID: id, ActorUserID: actor, AttachmentID: attachmentID, UploadIntentID: intentID, Owner: owner, EventType: eventType, Outcome: outcome, RequestID: requestID}
	if err := service.audit.RecordAuditEvent(ctx, event); err != nil && service.onAuditFailure != nil {
		service.onAuditFailure(ctx, event, err)
	}
}
