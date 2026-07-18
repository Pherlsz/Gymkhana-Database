package ocr

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/Pherlsz/Gymkhana-Database/internal/attachment"
	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

const (
	defaultTimeout       = 90 * time.Second
	defaultMaximumRate   = 10
	defaultMaximumUsage  = 500_000
	defaultRecoveryBatch = 100
	staleJobGrace        = 10 * time.Second
	applyLease           = 2 * time.Minute
)

type AuditFailureHandler func(context.Context, AuditEvent, error)

type ServiceOptions struct {
	Now                  func() time.Time
	Timeout              time.Duration
	MaximumRate          int
	MaximumProviderUsage int64
	MaximumSourceBytes   int64
	RecoveryBatch        int
	OnAuditFailure       AuditFailureHandler
}

type Service struct {
	store                Store
	jobs                 Jobs
	sources              SourceGateway
	targets              TargetGateway
	extractor            Extractor
	now                  func() time.Time
	timeout              time.Duration
	maximumRate          int
	maximumProviderUsage int64
	maximumSourceBytes   int64
	recoveryBatch        int
	onAuditFailure       AuditFailureHandler
}

func NewService(store Store, jobs Jobs, sources SourceGateway, targets TargetGateway, extractor Extractor, options ServiceOptions) (*Service, error) {
	if store == nil || jobs == nil || sources == nil || targets == nil || extractor == nil {
		return nil, ErrInvalidSetup
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.Timeout == 0 {
		options.Timeout = defaultTimeout
	}
	if options.MaximumRate == 0 {
		options.MaximumRate = defaultMaximumRate
	}
	if options.MaximumProviderUsage == 0 {
		options.MaximumProviderUsage = defaultMaximumUsage
	}
	if options.MaximumSourceBytes == 0 {
		options.MaximumSourceBytes = MaximumSourceBytes
	}
	if options.RecoveryBatch == 0 {
		options.RecoveryBatch = defaultRecoveryBatch
	}
	if options.Timeout < time.Second || options.Timeout > 5*time.Minute ||
		options.MaximumRate < 1 || options.MaximumRate > 1000 ||
		options.MaximumProviderUsage < 1 || options.MaximumProviderUsage > MaximumProviderUsage ||
		options.MaximumSourceBytes < 1 || options.MaximumSourceBytes > MaximumSourceBytes ||
		options.RecoveryBatch < 1 || options.RecoveryBatch > 1000 {
		return nil, ErrInvalidSetup
	}
	return &Service{
		store: store, jobs: jobs, sources: sources, targets: targets, extractor: extractor,
		now: options.Now, timeout: options.Timeout, maximumRate: options.MaximumRate,
		maximumProviderUsage: options.MaximumProviderUsage, maximumSourceBytes: options.MaximumSourceBytes,
		recoveryBatch: options.RecoveryBatch, onAuditFailure: options.OnAuditFailure,
	}, nil
}

func (service *Service) Capability() Capability {
	return Capability{
		Enabled: true, SupportedMIMEs: SupportedMIMEs(), MaximumSourceBytes: service.maximumSourceBytes,
		MaximumPages: MaximumPages, MaximumPixels: MaximumPixels, MaximumSuggestions: MaximumSuggestions,
		MaximumDuration: service.timeout, MaximumRequests: service.maximumRate,
		MaximumProviderUsage: service.maximumProviderUsage,
	}
}

func DefaultCapability() Capability {
	return Capability{
		SupportedMIMEs: SupportedMIMEs(), MaximumSourceBytes: MaximumSourceBytes,
		MaximumPages: MaximumPages, MaximumPixels: MaximumPixels, MaximumSuggestions: MaximumSuggestions,
		MaximumDuration: defaultTimeout, MaximumRequests: defaultMaximumRate,
		MaximumProviderUsage: defaultMaximumUsage,
	}
}

func (service *Service) StartJob(ctx context.Context, actor auth.Session, attachmentID attachment.Identifier, idempotencyKey string, retryOf *Identifier, requestID string) (Job, error) {
	user, err := service.authorize(ctx, actor)
	if err != nil {
		service.audit(ctx, auditActor(actor), nil, nil, AuditJobCreated, auditOutcome(err), nil, err, requestID)
		return Job{}, err
	}
	if attachmentID.IsZero() || !validIdempotencyKey(idempotencyKey) || retryOf != nil && retryOf.IsZero() {
		return Job{}, ErrInvalidInput
	}
	currentActor := auth.Session{User: user}
	if retryOf != nil {
		previous, err := service.store.GetJob(ctx, *retryOf, user.ID)
		if err != nil || !previous.State.Terminal() || previous.AttachmentID != attachmentID {
			if err == nil {
				err = ErrInvalidInput
			}
			return Job{}, err
		}
	}
	source, err := service.sources.GetForProcessing(ctx, currentActor, attachmentID)
	if err != nil {
		return Job{}, sourceError(err)
	}
	if !SupportedMIME(source.DetectedMIME) || source.ByteSize < 1 || source.ByteSize > service.maximumSourceBytes {
		return Job{}, ErrUnsafeSource
	}
	catalog, err := service.targets.Catalog(ctx, currentActor, source.Owner)
	if err != nil {
		return Job{}, err
	}
	fingerprint, err := jobFingerprint(user.ID, source, catalog.Fingerprint, retryOf)
	if err != nil {
		return Job{}, fmt.Errorf("fingerprint OCR job: %w", err)
	}
	id, err := NewIdentifier()
	if err != nil {
		return Job{}, fmt.Errorf("generate OCR job identifier: %w", err)
	}
	now := service.now().UTC()
	job, created, err := service.store.CreateJob(ctx, CreateJobInput{
		ID: id, OwnerUserID: user.ID, AttachmentID: attachmentID, RetryOfJobID: retryOf,
		IdempotencyKey: idempotencyKey, RequestFingerprint: fingerprint, SourceSHA256: source.SHA256,
		CatalogFingerprint: catalog.Fingerprint, SourceMIME: source.DetectedMIME, SourceBytes: source.ByteSize, Now: now,
	}, now.Truncate(time.Hour), service.maximumRate, service.maximumProviderUsage)
	if err != nil {
		service.audit(ctx, &user.ID, nil, nil, AuditJobCreated, auditOutcome(err), nil, err, requestID)
		return Job{}, err
	}
	if !created {
		if job.RiverJobID == 0 && !job.State.Terminal() {
			return service.enqueue(ctx, user, job, requestID)
		}
		return job, nil
	}
	service.audit(ctx, &user.ID, &job.ID, nil, AuditJobCreated, auth.AuditOutcomeSuccess, nil, nil, requestID)
	return service.enqueue(ctx, user, job, requestID)
}

func (service *Service) enqueue(ctx context.Context, user auth.User, job Job, requestID string) (Job, error) {
	jobID, err := service.jobs.EnqueueExtraction(ctx, job.ID)
	if err != nil {
		_, _ = service.store.FailJob(ctx, job.ID, "queue_unavailable", JobFailed, service.now().UTC())
		service.audit(ctx, &user.ID, &job.ID, nil, AuditJobFailed, auth.AuditOutcomeFailure, nil, ErrUnavailable, requestID)
		return Job{}, ErrUnavailable
	}
	updated, err := service.store.AttachRiverJob(ctx, job.ID, user.ID, jobID, service.now().UTC())
	if err != nil {
		_ = service.jobs.Cancel(ctx, jobID)
		if errors.Is(err, ErrConflict) {
			current, currentErr := service.store.GetJob(ctx, job.ID, user.ID)
			if currentErr == nil && current.RiverJobID > 0 {
				return current, nil
			}
		}
		_, _ = service.store.FailJob(ctx, job.ID, "queue_conflict", JobFailed, service.now().UTC())
		return Job{}, err
	}
	return updated, nil
}

func (service *Service) Jobs(ctx context.Context, actor auth.Session, limit, offset int, requestID string) (JobPage, error) {
	user, err := service.authorize(ctx, actor)
	if err != nil {
		return JobPage{}, err
	}
	if limit == 0 {
		limit = MaximumJobPage
	}
	if limit < 1 || limit > MaximumJobPage || offset < 0 || offset > 10_000 {
		return JobPage{}, ErrInvalidInput
	}
	page, err := service.store.ListJobs(ctx, user.ID, limit, offset)
	if err != nil {
		return JobPage{}, err
	}
	visible := page.Jobs[:0]
	currentActor := auth.Session{User: user}
	for _, job := range page.Jobs {
		if _, sourceErr := service.sources.GetForProcessing(ctx, currentActor, job.AttachmentID); sourceErr == nil {
			visible = append(visible, job)
		}
	}
	page.Jobs = visible
	page.Total = len(visible)
	service.audit(ctx, &user.ID, nil, nil, AuditJobRead, auth.AuditOutcomeSuccess, intPointer(len(visible)), nil, requestID)
	return page, nil
}

func (service *Service) Job(ctx context.Context, actor auth.Session, id Identifier, requestID string) (Job, error) {
	user, err := service.authorize(ctx, actor)
	if err != nil || id.IsZero() {
		if err == nil {
			err = ErrNotFound
		}
		return Job{}, err
	}
	job, err := service.store.GetJob(ctx, id, user.ID)
	if err != nil {
		return Job{}, err
	}
	if _, err := service.sources.GetForProcessing(ctx, auth.Session{User: user}, job.AttachmentID); err != nil {
		return Job{}, sourceError(err)
	}
	service.audit(ctx, &user.ID, &job.ID, nil, AuditJobRead, auth.AuditOutcomeSuccess, nil, nil, requestID)
	return job, nil
}

func (service *Service) CancelJob(ctx context.Context, actor auth.Session, id Identifier, requestID string) (Job, error) {
	user, err := service.authorize(ctx, actor)
	if err != nil || id.IsZero() {
		if err == nil {
			err = ErrNotFound
		}
		return Job{}, err
	}
	job, err := service.store.GetJob(ctx, id, user.ID)
	if err != nil {
		return Job{}, err
	}
	if _, err := service.sources.GetForProcessing(ctx, auth.Session{User: user}, job.AttachmentID); err != nil {
		return Job{}, sourceError(err)
	}
	job, err = service.store.RequestCancellation(ctx, id, user.ID, service.now().UTC())
	if err != nil {
		service.audit(ctx, &user.ID, &id, nil, AuditJobCancelled, auditOutcome(err), nil, err, requestID)
		return Job{}, err
	}
	if job.RiverJobID > 0 {
		if err := service.jobs.Cancel(ctx, job.RiverJobID); err != nil {
			return Job{}, err
		}
	}
	service.audit(ctx, &user.ID, &id, nil, AuditJobCancelled, auth.AuditOutcomeSuccess, nil, nil, requestID)
	return job, nil
}

func (service *Service) Events(ctx context.Context, actor auth.Session, id Identifier, after int64, limit int) (EventPage, error) {
	job, err := service.Job(ctx, actor, id, "events")
	if err != nil {
		return EventPage{}, err
	}
	if after < 0 {
		return EventPage{}, ErrInvalidInput
	}
	if limit == 0 {
		limit = MaximumEventPage
	}
	if limit < 1 || limit > MaximumEventPage {
		return EventPage{}, ErrInvalidInput
	}
	return service.store.ListEvents(ctx, job.ID, job.OwnerUserID, after, limit)
}

func (service *Service) Suggestions(ctx context.Context, actor auth.Session, jobID Identifier, limit, offset int, requestID string) (SuggestionPage, error) {
	user, err := service.authorize(ctx, actor)
	if err != nil {
		return SuggestionPage{}, err
	}
	job, err := service.store.GetJob(ctx, jobID, user.ID)
	if err != nil {
		return SuggestionPage{}, err
	}
	source, err := service.sources.GetForProcessing(ctx, auth.Session{User: user}, job.AttachmentID)
	if err != nil {
		return SuggestionPage{}, sourceError(err)
	}
	if limit == 0 {
		limit = MaximumSuggestions
	}
	if limit < 1 || limit > MaximumSuggestions || offset < 0 || offset > 10_000 {
		return SuggestionPage{}, ErrInvalidInput
	}
	values, total, err := service.store.ListSuggestions(ctx, jobID, user.ID, limit, offset)
	if err != nil {
		return SuggestionPage{}, err
	}
	catalog, err := service.targets.Catalog(ctx, auth.Session{User: user}, source.Owner)
	if err != nil {
		return SuggestionPage{}, err
	}
	fields := catalogFields(catalog)
	views := make([]SuggestionView, 0, len(values))
	for _, suggestion := range values {
		view := SuggestionView{Suggestion: suggestion, Stale: true}
		field, ok := fields[suggestion.FieldKey]
		if ok && field.Target == suggestion.Target && field.Kind == suggestion.Kind {
			current, currentErr := service.targets.CurrentField(ctx, auth.Session{User: user}, field)
			if currentErr != nil {
				return SuggestionPage{}, currentErr
			}
			view.CurrentValue = current.Value
			view.CurrentVersion = current.Version
			view.Stale = current.Version != suggestion.TargetVersion
		}
		views = append(views, view)
	}
	service.audit(ctx, &user.ID, &jobID, nil, AuditSuggestionsRead, auth.AuditOutcomeSuccess, intPointer(len(views)), nil, requestID)
	return SuggestionPage{Suggestions: views, Total: total, Limit: limit, Offset: offset}, nil
}

func (service *Service) ReviewSuggestion(ctx context.Context, actor auth.Session, id Identifier, input ReviewInput, requestID string) (Suggestion, error) {
	user, err := service.authorize(ctx, actor)
	if err != nil || id.IsZero() || input.Version < 1 || input.Action != ReviewAccept && input.Action != ReviewReject {
		if err == nil {
			err = ErrInvalidInput
		}
		return Suggestion{}, err
	}
	suggestion, job, err := service.store.GetSuggestion(ctx, id, user.ID)
	if err != nil {
		return Suggestion{}, err
	}
	source, err := service.sources.GetForProcessing(ctx, auth.Session{User: user}, job.AttachmentID)
	if err != nil {
		return Suggestion{}, sourceError(err)
	}
	catalog, err := service.targets.Catalog(ctx, auth.Session{User: user}, source.Owner)
	if err != nil {
		return Suggestion{}, err
	}
	field, ok := catalogFields(catalog)[suggestion.FieldKey]
	if !ok || field.Target != suggestion.Target || field.Kind != suggestion.Kind {
		return Suggestion{}, ErrStaleTarget
	}
	current, err := service.targets.CurrentField(ctx, auth.Session{User: user}, field)
	if err != nil {
		return Suggestion{}, err
	}
	var reviewed *string
	if input.Action == ReviewAccept {
		value := suggestion.ProposedValue
		if input.Value != nil {
			value = *input.Value
		}
		normalized, err := normalizeValue(field.Kind, value, !field.Required)
		if err != nil {
			return Suggestion{}, err
		}
		reviewed = &normalized
	} else if input.Value != nil {
		return Suggestion{}, ErrInvalidInput
	}
	updated, err := service.store.ReviewSuggestion(ctx, ReviewSuggestionInput{
		SuggestionID: id, OwnerUserID: user.ID, Action: input.Action, Value: reviewed,
		TargetVersion: current.Version, Version: input.Version, Now: service.now().UTC(),
	})
	service.auditReview(ctx, user.ID, id, suggestion.FieldKey, input.Action, auditOutcome(err), err, requestID)
	return updated, err
}

func (service *Service) Apply(ctx context.Context, actor auth.Session, jobID Identifier, selections []ApplySelection, idempotencyKey, requestID string) (ApplyReceipt, error) {
	user, err := service.authorize(ctx, actor)
	if err != nil || jobID.IsZero() || !validIdempotencyKey(idempotencyKey) || len(selections) == 0 || len(selections) > MaximumSuggestions {
		if err == nil {
			err = ErrInvalidInput
		}
		return ApplyReceipt{}, err
	}
	job, err := service.store.GetJob(ctx, jobID, user.ID)
	if err != nil || job.State != JobCompleted {
		if err == nil {
			err = ErrInvalidState
		}
		return ApplyReceipt{}, err
	}
	source, err := service.sources.GetForProcessing(ctx, auth.Session{User: user}, job.AttachmentID)
	if err != nil {
		return ApplyReceipt{}, sourceError(err)
	}
	catalog, err := service.targets.Catalog(ctx, auth.Session{User: user}, source.Owner)
	if err != nil {
		return ApplyReceipt{}, err
	}
	normalizedSelections, fingerprint, err := applyFingerprint(jobID, selections)
	if err != nil {
		return ApplyReceipt{}, err
	}
	receiptID, err := NewIdentifier()
	if err != nil {
		return ApplyReceipt{}, fmt.Errorf("generate OCR apply receipt identifier: %w", err)
	}
	now := service.now().UTC()
	receipt, created, err := service.store.CreateApplyReceipt(ctx, CreateApplyReceiptInput{
		ID: receiptID, JobID: jobID, OwnerUserID: user.ID, IdempotencyKey: idempotencyKey,
		RequestFingerprint: fingerprint, Now: now,
	})
	if err != nil {
		return ApplyReceipt{}, err
	}
	if !created {
		if receipt.State == ApplyCompleted {
			return receipt, nil
		}
		receipt, created, err = service.store.ResumeApplyReceipt(ctx, receipt.ID, user.ID, now.Add(-applyLease), now)
		if err != nil || !created {
			if err == nil {
				err = ErrConflict
			}
			return ApplyReceipt{}, err
		}
	}
	completedSuggestions := make(map[Identifier]struct{}, len(receipt.Results))
	for _, result := range receipt.Results {
		completedSuggestions[result.SuggestionID] = struct{}{}
	}
	pendingSelections := make([]ApplySelection, 0, len(normalizedSelections))
	for _, selection := range normalizedSelections {
		if _, completed := completedSuggestions[selection.SuggestionID]; !completed {
			pendingSelections = append(pendingSelections, selection)
		}
	}
	if len(pendingSelections) == 0 {
		return service.store.CompleteApplyReceipt(ctx, receipt.ID, user.ID, service.now().UTC())
	}
	suggestions, err := service.store.GetSuggestionsForApply(ctx, jobID, user.ID, pendingSelections)
	if err != nil {
		return ApplyReceipt{}, err
	}
	fields := catalogFields(catalog)
	groups, err := buildApplyGroups(suggestions, fields)
	if err != nil {
		if errors.Is(err, ErrStaleTarget) {
			inputs := make([]ApplyResultInput, 0, len(suggestions))
			for _, suggestion := range suggestions {
				inputs = append(inputs, ApplyResultInput{SuggestionID: suggestion.ID, Outcome: ApplyStale, ErrorCode: publicErrorCode(err)})
			}
			if _, saveErr := service.store.SaveApplyResults(ctx, receipt.ID, inputs, service.now().UTC()); saveErr != nil {
				return ApplyReceipt{}, saveErr
			}
			return service.store.CompleteApplyReceipt(ctx, receipt.ID, user.ID, service.now().UTC())
		}
		return ApplyReceipt{}, err
	}
	service.audit(ctx, &user.ID, &jobID, nil, AuditApplicationStarted, auth.AuditOutcomeSuccess, intPointer(len(suggestions)), nil, requestID)
	for _, group := range groups {
		newVersion, applyErr := service.applyGroup(ctx, auth.Session{User: user}, group, requestID)
		inputs := make([]ApplyResultInput, 0, len(group.Changes))
		outcome, code := ApplyApplied, ""
		var version *int64
		if applyErr == nil {
			version = &newVersion
		} else if errors.Is(applyErr, ErrStaleTarget) {
			outcome, code = ApplyStale, publicErrorCode(applyErr)
		} else {
			outcome, code = ApplyFailed, publicErrorCode(applyErr)
		}
		for _, change := range group.Changes {
			inputs = append(inputs, ApplyResultInput{SuggestionID: change.SuggestionID, Outcome: outcome, TargetVersion: version, ErrorCode: code})
		}
		if _, err := service.store.SaveApplyResults(ctx, receipt.ID, inputs, service.now().UTC()); err != nil {
			return ApplyReceipt{}, err
		}
	}
	receipt, err = service.store.CompleteApplyReceipt(ctx, receipt.ID, user.ID, service.now().UTC())
	service.audit(ctx, &user.ID, &jobID, nil, AuditApplicationComplete, auditOutcome(err), intPointer(len(suggestions)), err, requestID)
	return receipt, err
}

type applyGroup struct {
	Target          TargetReference
	ExpectedVersion int64
	Changes         []ApprovedChange
}

func buildApplyGroups(suggestions []Suggestion, fields map[string]FieldSchema) ([]applyGroup, error) {
	groups := make(map[string]*applyGroup)
	order := make([]string, 0)
	for _, suggestion := range suggestions {
		if suggestion.ReviewState != ReviewAccepted || suggestion.ReviewedValue == nil {
			return nil, ErrInvalidState
		}
		field, ok := fields[suggestion.FieldKey]
		if !ok || field.Target != suggestion.Target || field.Kind != suggestion.Kind {
			return nil, ErrStaleTarget
		}
		key := string(suggestion.Target.Kind) + ":" + suggestion.Target.ID.String() + ":" + fmt.Sprint(suggestion.TargetVersion)
		group := groups[key]
		if group == nil {
			group = &applyGroup{Target: suggestion.Target, ExpectedVersion: suggestion.TargetVersion}
			groups[key] = group
			order = append(order, key)
		}
		group.Changes = append(group.Changes, ApprovedChange{SuggestionID: suggestion.ID, Field: field, Value: *suggestion.ReviewedValue})
	}
	result := make([]applyGroup, 0, len(order))
	for _, key := range order {
		result = append(result, *groups[key])
	}
	return result, nil
}

func (service *Service) applyGroup(ctx context.Context, actor auth.Session, group applyGroup, requestID string) (int64, error) {
	allEqual := true
	currentVersion := int64(0)
	for _, change := range group.Changes {
		current, err := service.targets.CurrentField(ctx, actor, change.Field)
		if err != nil {
			return 0, err
		}
		if currentVersion == 0 {
			currentVersion = current.Version
		} else if current.Version != currentVersion {
			return 0, ErrStaleTarget
		}
		allEqual = allEqual && current.Value == change.Value
	}
	if allEqual {
		return currentVersion, nil
	}
	if currentVersion != group.ExpectedVersion {
		return 0, ErrStaleTarget
	}
	return service.targets.ApplyTarget(ctx, actor, group.Target, group.ExpectedVersion, group.Changes, requestID)
}

func (service *Service) RunJob(ctx context.Context, id Identifier) (resultErr error) {
	if id.IsZero() {
		return ErrInvalidInput
	}
	job, err := service.store.GetJobForWorker(ctx, id)
	if err != nil {
		return err
	}
	user, err := service.store.CurrentUser(ctx, job.OwnerUserID)
	if err != nil || !user.Active || !user.Role.CanReadAttachments() {
		if err == nil {
			err = ErrForbidden
		}
		_, _ = service.store.FailJob(ctx, id, publicErrorCode(err), JobFailed, service.now().UTC())
		return nil
	}
	job, claimed, err := service.store.ClaimJob(ctx, id, service.now().UTC())
	if err != nil || !claimed {
		return err
	}
	service.audit(ctx, &user.ID, &job.ID, nil, AuditJobStarted, auth.AuditOutcomeSuccess, nil, nil, "worker")
	providerStarted := false
	defer func() {
		if resultErr == nil {
			return
		}
		if !providerStarted && errors.Is(resultErr, ErrUnavailable) && job.AttemptCount < MaximumAttempts {
			return
		}
		state := JobFailed
		if errors.Is(resultErr, ErrCancelled) || errors.Is(resultErr, context.Canceled) {
			state = JobCancelled
		}
		code := publicErrorCode(resultErr)
		finalCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = service.store.FailJob(finalCtx, id, code, state, service.now().UTC())
		event := AuditJobFailed
		if state == JobCancelled {
			event = AuditJobCancelled
		}
		service.audit(finalCtx, &user.ID, &job.ID, nil, event, auth.AuditOutcomeFailure, nil, resultErr, "worker")
	}()
	runCtx, cancel := context.WithTimeout(ctx, service.timeout)
	defer cancel()
	actor := auth.Session{User: user}
	source, reader, err := service.sources.OpenForProcessing(runCtx, actor, job.AttachmentID)
	if err != nil {
		return sourceError(err)
	}
	if source.SHA256 != job.SourceSHA256 || source.DetectedMIME != job.SourceMIME || source.ByteSize != job.SourceBytes {
		_ = reader.Close()
		return ErrUnsafeSource
	}
	catalog, err := service.targets.Catalog(runCtx, actor, source.Owner)
	if err != nil {
		_ = reader.Close()
		return err
	}
	if catalog.Fingerprint != job.CatalogFingerprint {
		_ = reader.Close()
		return ErrStaleTarget
	}
	validated, validationErr := ValidateSource(reader, source.DetectedMIME, source.ByteSize, source.SHA256, service.maximumSourceBytes)
	closeErr := reader.Close()
	if validationErr != nil {
		return validationErr
	}
	if closeErr != nil {
		return fmt.Errorf("close OCR source: %w", closeErr)
	}
	if _, err := service.store.RecordSourceValidated(runCtx, job.ID, validated.PageCount, validated.PixelCount, service.now().UTC()); err != nil {
		return err
	}
	if current, err := service.store.GetJobForWorker(runCtx, id); err != nil || current.CancelRequestedAt != nil || current.State == JobCancelled {
		if err != nil {
			return err
		}
		return ErrCancelled
	}
	currentSource, err := service.sources.GetForProcessing(runCtx, actor, job.AttachmentID)
	if err != nil {
		return sourceError(err)
	}
	if currentSource.SHA256 != job.SourceSHA256 || currentSource.DetectedMIME != job.SourceMIME ||
		currentSource.ByteSize != job.SourceBytes || currentSource.Owner != source.Owner {
		return ErrUnsafeSource
	}
	providerFields := make([]ProviderField, 0, len(catalog.Fields))
	for _, field := range catalog.Fields {
		providerFields = append(providerFields, ProviderField{Key: field.Key, Label: field.Label, Kind: field.Kind, Required: field.Required})
	}
	providerStarted = true
	response, err := service.extractor.Extract(runCtx, ExtractionRequest{
		SchemaVersion: SchemaVersion, MIME: source.DetectedMIME, ByteSize: source.ByteSize, SHA256: source.SHA256,
		PageCount: validated.PageCount, PixelCount: validated.PixelCount, Fields: providerFields, Source: bytes.NewReader(validated.Bytes),
	})
	validated.Bytes = nil
	if err != nil {
		return providerError(runCtx, err)
	}
	if response.Usage < 0 || response.Usage > MaximumProviderUsage {
		return ErrMalformedProvider
	}
	// A successful provider response has already consumed billable capacity. Use a
	// short detached context so a concurrent request cancellation cannot bypass
	// durable usage accounting. The cancellation is honored immediately below.
	accountingCtx, accountingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	accountedJob, exceeded, err := service.store.AddProviderUsage(
		accountingCtx, id, user.ID, response.Usage, service.maximumProviderUsage, service.now().UTC(),
	)
	accountingCancel()
	if err != nil {
		return err
	}
	if accountedJob.CancelRequestedAt != nil || accountedJob.State == JobCancelled {
		return ErrCancelled
	}
	if runErr := runCtx.Err(); runErr != nil {
		return providerError(runCtx, runErr)
	}
	if exceeded {
		return ErrQuotaExceeded
	}
	suggestions, err := normalizeProviderSuggestions(response, catalog, validated.PageCount)
	if err != nil {
		return err
	}
	if current, err := service.store.GetJobForWorker(runCtx, id); err != nil || current.CancelRequestedAt != nil || current.State == JobCancelled {
		if err != nil {
			return err
		}
		return ErrCancelled
	}
	completed, err := service.store.CompleteJob(runCtx, CompleteJobInput{
		JobID: id, PageCount: validated.PageCount, PixelCount: validated.PixelCount,
		Suggestions: suggestions, Now: service.now().UTC(),
	})
	if err != nil {
		return err
	}
	service.audit(ctx, &user.ID, &completed.ID, nil, AuditJobCompleted, auth.AuditOutcomeSuccess, intPointer(len(suggestions)), nil, "worker")
	return nil
}

func (service *Service) RecoverStaleJobs(ctx context.Context) (int, error) {
	now := service.now().UTC()
	affected, err := service.store.RecoverStaleJobs(ctx, now.Add(-service.timeout-staleJobGrace), now, service.recoveryBatch)
	service.audit(ctx, nil, nil, nil, AuditJobRecovered, auditOutcome(err), intPointer(affected), err, "worker-recovery")
	return affected, err
}

func (service *Service) authorize(ctx context.Context, actor auth.Session) (auth.User, error) {
	if actor.User.ID == (auth.Identifier{}) || !actor.User.Active || !actor.User.Role.CanReadAttachments() {
		return auth.User{}, ErrForbidden
	}
	current, err := service.store.CurrentUser(ctx, actor.User.ID)
	if err != nil {
		return auth.User{}, err
	}
	if !current.Active || !current.Role.CanReadAttachments() {
		return auth.User{}, ErrForbidden
	}
	return current, nil
}

func jobFingerprint(owner auth.Identifier, source attachment.Attachment, catalog [32]byte, retry *Identifier) ([32]byte, error) {
	type payload struct {
		Owner      string  `json:"owner"`
		Attachment string  `json:"attachment"`
		SourceSHA  string  `json:"source_sha"`
		Catalog    string  `json:"catalog"`
		Retry      *string `json:"retry,omitempty"`
		Schema     string  `json:"schema"`
	}
	value := payload{Owner: owner.String(), Attachment: source.ID.String(), SourceSHA: fmt.Sprintf("%x", source.SHA256), Catalog: fmt.Sprintf("%x", catalog), Schema: SchemaVersion}
	if retry != nil {
		text := retry.String()
		value.Retry = &text
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(encoded), nil
}

func applyFingerprint(jobID Identifier, selections []ApplySelection) ([]ApplySelection, [32]byte, error) {
	normalized := append([]ApplySelection(nil), selections...)
	sort.Slice(normalized, func(left, right int) bool {
		return normalized[left].SuggestionID.String() < normalized[right].SuggestionID.String()
	})
	for index, selection := range normalized {
		if selection.SuggestionID.IsZero() || selection.Version < 1 || index > 0 && normalized[index-1].SuggestionID == selection.SuggestionID {
			return nil, [32]byte{}, ErrInvalidInput
		}
	}
	payload := struct {
		Job        string           `json:"job"`
		Selections []ApplySelection `json:"selections"`
	}{Job: jobID.String(), Selections: normalized}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, [32]byte{}, err
	}
	return normalized, sha256.Sum256(encoded), nil
}

func catalogFields(catalog Catalog) map[string]FieldSchema {
	result := make(map[string]FieldSchema, len(catalog.Fields))
	for _, field := range catalog.Fields {
		result[field.Key] = field
	}
	return result
}

func validIdempotencyKey(value string) bool {
	if len(value) < MinimumIdempotencyLength || len(value) > MaximumIdempotencyLength || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsLetter(character) || unicode.IsDigit(character) || strings.ContainsRune("._:-", character) {
			continue
		}
		return false
	}
	return true
}

func providerError(ctx context.Context, err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
		return ErrCancelled
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, ErrTimeout) {
		return ErrTimeout
	}
	if errors.Is(err, ErrMalformedProvider) || errors.Is(err, ErrQuotaExceeded) || errors.Is(err, ErrUnsafeSource) {
		return err
	}
	return ErrUnavailable
}

func sourceError(err error) error {
	switch {
	case errors.Is(err, attachment.ErrForbidden):
		return ErrForbidden
	case errors.Is(err, attachment.ErrAttachmentNotFound), errors.Is(err, attachment.ErrUploadObjectNotFound):
		return ErrNotFound
	case errors.Is(err, attachment.ErrInvalidState), errors.Is(err, attachment.ErrUnsupportedFile), errors.Is(err, attachment.ErrMIMEMismatch):
		return ErrUnsafeSource
	case errors.Is(err, attachment.ErrStorageUnavailable):
		return ErrUnavailable
	default:
		return err
	}
}

func publicErrorCode(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrCancelled), errors.Is(err, context.Canceled):
		return "cancelled"
	case errors.Is(err, ErrConflict):
		return "conflict"
	case errors.Is(err, ErrForbidden):
		return "forbidden"
	case errors.Is(err, ErrInvalidInput):
		return "invalid_input"
	case errors.Is(err, ErrInvalidState):
		return "invalid_state"
	case errors.Is(err, ErrMalformedProvider):
		return "malformed_provider"
	case errors.Is(err, ErrNotFound):
		return "not_found"
	case errors.Is(err, ErrQuotaExceeded):
		return "quota_exceeded"
	case errors.Is(err, ErrRateLimited):
		return "rate_limited"
	case errors.Is(err, ErrStaleTarget):
		return "stale_target"
	case errors.Is(err, ErrTimeout), errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, ErrUnavailable):
		return "unavailable"
	case errors.Is(err, ErrUnsafeSource):
		return "unsafe_source"
	default:
		return "internal_error"
	}
}

func auditOutcome(err error) auth.AuditOutcome {
	if err == nil {
		return auth.AuditOutcomeSuccess
	}
	if errors.Is(err, ErrForbidden) || errors.Is(err, ErrNotFound) || errors.Is(err, ErrConflict) ||
		errors.Is(err, ErrInvalidInput) || errors.Is(err, ErrInvalidState) || errors.Is(err, ErrQuotaExceeded) ||
		errors.Is(err, ErrRateLimited) || errors.Is(err, ErrStaleTarget) || errors.Is(err, ErrUnsafeSource) {
		return auth.AuditOutcomeDenied
	}
	return auth.AuditOutcomeFailure
}

func (service *Service) audit(ctx context.Context, actor *auth.Identifier, jobID, suggestionID *Identifier, eventType AuditEventType, outcome auth.AuditOutcome, affected *int, eventErr error, requestID string) {
	id, err := NewIdentifier()
	if err != nil {
		service.reportAuditFailure(ctx, AuditEvent{ActorUserID: actor, JobID: jobID, SuggestionID: suggestionID, EventType: eventType, Outcome: outcome}, err)
		return
	}
	event := AuditEvent{
		ID: id, ActorUserID: actor, JobID: jobID, SuggestionID: suggestionID, EventType: eventType,
		Outcome: outcome, AffectedCount: affected, ErrorCode: publicErrorCode(eventErr), RequestID: requestIDValue(requestID), CreatedAt: service.now().UTC(),
	}
	if err := service.store.SaveAudit(ctx, event); err != nil {
		service.reportAuditFailure(ctx, event, err)
	}
}

func (service *Service) auditReview(ctx context.Context, actor auth.Identifier, suggestionID Identifier, fieldKey string, action ReviewAction, outcome auth.AuditOutcome, eventErr error, requestID string) {
	id, err := NewIdentifier()
	event := AuditEvent{
		ID: id, ActorUserID: &actor, SuggestionID: &suggestionID, FieldKey: fieldKey, ReviewAction: action,
		EventType: AuditSuggestionReviewed, Outcome: outcome, ErrorCode: publicErrorCode(eventErr),
		RequestID: requestIDValue(requestID), CreatedAt: service.now().UTC(),
	}
	if err != nil {
		service.reportAuditFailure(ctx, event, err)
		return
	}
	if err := service.store.SaveAudit(ctx, event); err != nil {
		service.reportAuditFailure(ctx, event, err)
	}
}

func (service *Service) reportAuditFailure(ctx context.Context, event AuditEvent, err error) {
	if service.onAuditFailure != nil {
		service.onAuditFailure(ctx, event, err)
	}
}

func auditActor(actor auth.Session) *auth.Identifier {
	if actor.User.ID == (auth.Identifier{}) {
		return nil
	}
	value := actor.User.ID
	return &value
}

func requestIDValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 128 {
		return value[:128]
	}
	return value
}

func intPointer(value int) *int { return &value }
