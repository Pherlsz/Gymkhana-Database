package googleforms

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/operations"
)

type AuditFailureHandler func(context.Context, AuditEvent, error)

type ServiceOptions struct {
	Enabled          bool
	ImportRetention  time.Duration
	ResponsePageSize int
	SyncBatchSize    int
	Now              func() time.Time
	OnAuditFailure   AuditFailureHandler
	// ProductEnabled gates worker sync when product on/off lives outside env.
	ProductEnabled func(context.Context) bool
}

type Service struct {
	store            Store
	provider         Provider
	cipher           *TokenCipher
	jobs             Jobs
	operations       Operations
	enabled          bool
	importRetention  time.Duration
	responsePageSize int
	syncBatchSize    int
	now              func() time.Time
	onAuditFailure   AuditFailureHandler
	productEnabled   func(context.Context) bool
}

type OAuthStart struct {
	AuthorizationURL string
	ExpiresAt        time.Time
}

func NewService(store Store, provider Provider, cipher *TokenCipher, jobs Jobs, operationService Operations, options ServiceOptions) (*Service, error) {
	if store == nil || jobs == nil || operationService == nil || options.Enabled && (provider == nil || cipher == nil) {
		return nil, fmt.Errorf("configure google forms: %w", ErrInvalidInput)
	}
	if options.ImportRetention == 0 {
		options.ImportRetention = 24 * time.Hour
	}
	if options.ResponsePageSize == 0 {
		options.ResponsePageSize = 100
	}
	if options.SyncBatchSize == 0 {
		options.SyncBatchSize = 25
	}
	if options.ImportRetention < time.Hour || options.ImportRetention > 7*24*time.Hour ||
		options.ResponsePageSize < 1 || options.ResponsePageSize > 500 ||
		options.SyncBatchSize < 1 || options.SyncBatchSize > 100 {
		return nil, fmt.Errorf("configure google forms limits: %w", ErrInvalidInput)
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	return &Service{
		store: store, provider: provider, cipher: cipher, jobs: jobs, operations: operationService,
		enabled: options.Enabled, importRetention: options.ImportRetention,
		responsePageSize: options.ResponsePageSize, syncBatchSize: options.SyncBatchSize,
		now: options.Now, onAuditFailure: options.OnAuditFailure, productEnabled: options.ProductEnabled,
	}, nil
}

func (service *Service) Enabled() bool {
	return service != nil && service.enabled
}

func (service *Service) productOn(ctx context.Context) bool {
	if service == nil || !service.enabled {
		return false
	}
	if service.productEnabled == nil {
		return true
	}
	return service.productEnabled(ctx)
}

func (service *Service) BeginOAuth(ctx context.Context, actor auth.Session, returnPath, requestID string) (OAuthStart, error) {
	if err := service.authorize(actor); err != nil {
		service.audit(ctx, &actor.User.ID, nil, nil, nil, AuditConnectionStarted, auth.AuditOutcomeDenied, nil, requestID)
		return OAuthStart{}, err
	}
	returnPath, err := safeReturnPath(returnPath)
	if err != nil {
		return OAuthStart{}, err
	}
	state, err := randomToken(32)
	if err != nil {
		return OAuthStart{}, fmt.Errorf("generate oauth state: %w", err)
	}
	verifier, err := randomToken(48)
	if err != nil {
		return OAuthStart{}, fmt.Errorf("generate oauth verifier: %w", err)
	}
	hash := sha256.Sum256([]byte(state))
	sealed, err := service.cipher.Seal([]byte(verifier), oauthVerifierAAD(hash))
	if err != nil {
		return OAuthStart{}, err
	}
	now := service.now().UTC()
	value := OAuthState{
		StateHash: hash, OwnerUserID: actor.User.ID, SessionID: actor.ID,
		VerifierCiphertext: sealed.Data, VerifierNonce: sealed.Nonce,
		TokenKeyVersion: sealed.KeyVersion, ReturnPath: returnPath,
		CreatedAt: now, ExpiresAt: now.Add(OAuthStateTTL),
	}
	if err := service.store.SaveOAuthState(ctx, value); err != nil {
		service.audit(ctx, &actor.User.ID, nil, nil, nil, AuditConnectionStarted, auth.AuditOutcomeFailure, nil, requestID)
		return OAuthStart{}, err
	}
	authorizationURL, err := service.provider.AuthorizationURL(state, codeChallenge(verifier))
	if err != nil {
		return OAuthStart{}, err
	}
	service.audit(ctx, &actor.User.ID, nil, nil, nil, AuditConnectionStarted, auth.AuditOutcomeSuccess, nil, requestID)
	return OAuthStart{AuthorizationURL: authorizationURL, ExpiresAt: value.ExpiresAt}, nil
}

func (service *Service) CompleteOAuth(ctx context.Context, actor auth.Session, state, code, requestID string) (Connection, string, error) {
	if err := service.authorize(actor); err != nil {
		service.audit(ctx, &actor.User.ID, nil, nil, nil, AuditConnectionFailed, auth.AuditOutcomeDenied, nil, requestID)
		return Connection{}, "", err
	}
	state = strings.TrimSpace(state)
	code = strings.TrimSpace(code)
	if state == "" || len(state) > 256 || code == "" || len(code) > 4096 {
		return Connection{}, "", ErrOAuthState
	}
	hash := sha256.Sum256([]byte(state))
	now := service.now().UTC()
	stored, err := service.store.ConsumeOAuthState(ctx, hash, actor.User.ID, actor.ID, now)
	if err != nil {
		service.audit(ctx, &actor.User.ID, nil, nil, nil, AuditConnectionFailed, auth.AuditOutcomeFailure, nil, requestID)
		return Connection{}, "", err
	}
	verifier, err := service.cipher.Open(Ciphertext{
		Data: stored.VerifierCiphertext, Nonce: stored.VerifierNonce, KeyVersion: stored.TokenKeyVersion,
	}, oauthVerifierAAD(hash))
	if err != nil {
		service.audit(ctx, &actor.User.ID, nil, nil, nil, AuditConnectionFailed, auth.AuditOutcomeFailure, nil, requestID)
		return Connection{}, "", err
	}
	token, err := service.provider.Exchange(ctx, code, string(verifier))
	if err != nil {
		service.audit(ctx, &actor.User.ID, nil, nil, nil, AuditConnectionFailed, auth.AuditOutcomeFailure, nil, requestID)
		return Connection{}, "", err
	}
	if !hasRequiredScopes(token.Scopes) {
		_ = service.provider.Revoke(ctx, token.RefreshToken)
		service.audit(ctx, &actor.User.ID, nil, nil, nil, AuditConnectionFailed, auth.AuditOutcomeFailure, nil, requestID)
		return Connection{}, "", ErrOAuthScopes
	}
	sealed, err := service.cipher.Seal([]byte(token.RefreshToken), refreshTokenAAD(actor.User.ID))
	if err != nil {
		return Connection{}, "", err
	}
	id, err := NewIdentifier()
	if err != nil {
		return Connection{}, "", fmt.Errorf("generate connection identifier: %w", err)
	}
	connectedAt := now
	connection, err := service.store.UpsertConnection(ctx, Connection{
		ID: id, OwnerUserID: actor.User.ID, State: ConnectionActive,
		RefreshTokenCiphertext: sealed.Data, RefreshTokenNonce: sealed.Nonce,
		TokenKeyVersion: sealed.KeyVersion, GrantedScopes: canonicalScopes(token.Scopes),
		ConnectedAt: &connectedAt, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		_ = service.provider.Revoke(ctx, token.RefreshToken)
		service.audit(ctx, &actor.User.ID, nil, nil, nil, AuditConnectionFailed, auth.AuditOutcomeFailure, nil, requestID)
		return Connection{}, "", err
	}
	service.audit(ctx, &actor.User.ID, &connection.ID, nil, nil, AuditConnectionCompleted, auth.AuditOutcomeSuccess, nil, requestID)
	return connection, stored.ReturnPath, nil
}

func (service *Service) RejectOAuth(ctx context.Context, actor auth.Session, state, requestID string) (string, error) {
	if err := service.authorize(actor); err != nil {
		return "", err
	}
	state = strings.TrimSpace(state)
	if state == "" || len(state) > 256 {
		return "", ErrOAuthState
	}
	hash := sha256.Sum256([]byte(state))
	stored, err := service.store.ConsumeOAuthState(ctx, hash, actor.User.ID, actor.ID, service.now().UTC())
	if err != nil {
		return "", err
	}
	service.audit(ctx, &actor.User.ID, nil, nil, nil, AuditConnectionFailed, auth.AuditOutcomeFailure, nil, requestID)
	return stored.ReturnPath, nil
}

func (service *Service) GetConnection(ctx context.Context, actor auth.Session) (Connection, error) {
	if err := service.authorize(actor); err != nil {
		return Connection{}, err
	}
	return service.store.GetConnection(ctx, actor.User.ID)
}

func (service *Service) Disconnect(ctx context.Context, actor auth.Session, version int64, requestID string) (Connection, error) {
	if err := service.authorize(actor); err != nil {
		service.audit(ctx, &actor.User.ID, nil, nil, nil, AuditConnectionDisconnected, auth.AuditOutcomeDenied, nil, requestID)
		return Connection{}, err
	}
	connection, err := service.store.GetConnection(ctx, actor.User.ID)
	if err != nil {
		return Connection{}, err
	}
	if connection.Version != version {
		return Connection{}, ErrConflict
	}
	if connection.State == ConnectionActive {
		refreshToken, openErr := service.openRefreshToken(connection)
		if openErr != nil {
			service.audit(ctx, &actor.User.ID, &connection.ID, nil, nil, AuditConnectionDisconnected, auth.AuditOutcomeFailure, nil, requestID)
		} else if revokeErr := service.provider.Revoke(ctx, refreshToken); revokeErr != nil && !errors.Is(revokeErr, ErrNeedsReauth) {
			service.audit(ctx, &actor.User.ID, &connection.ID, nil, nil, AuditConnectionDisconnected, auth.AuditOutcomeFailure, nil, requestID)
		}
	}
	disconnected, err := service.store.DisconnectConnection(ctx, actor.User.ID, version, service.now().UTC())
	if err != nil {
		return Connection{}, err
	}
	service.audit(ctx, &actor.User.ID, &connection.ID, nil, nil, AuditConnectionDisconnected, auth.AuditOutcomeSuccess, nil, requestID)
	return disconnected, nil
}

func (service *Service) CreateSource(ctx context.Context, actor auth.Session, input CreateSourceInput, requestID string) (Source, error) {
	if err := service.authorize(actor); err != nil {
		service.audit(ctx, &actor.User.ID, nil, nil, nil, AuditSourceCreated, auth.AuditOutcomeDenied, nil, requestID)
		return Source{}, err
	}
	formID, err := providerFormID(input.FormReference)
	if err != nil || !input.Module.Valid() {
		return Source{}, ErrInvalidInput
	}
	connection, err := service.store.GetConnection(ctx, actor.User.ID)
	if err != nil {
		return Source{}, err
	}
	if connection.State != ConnectionActive {
		return Source{}, ErrNeedsReauth
	}
	refreshToken, err := service.openRefreshToken(connection)
	if err != nil {
		return Source{}, err
	}
	form, err := service.provider.GetForm(ctx, refreshToken, formID)
	if err != nil {
		return Source{}, service.handleProviderConnectionError(ctx, actor.User.ID, err)
	}
	if form.ID != formID || supportedQuestionCount(form.Questions) == 0 {
		return Source{}, ErrUnsupportedForm
	}
	catalog, err := service.moduleCatalog(ctx, actor, input.Module)
	if err != nil || !catalog.CanImport {
		return Source{}, ErrForbidden
	}
	id, err := NewIdentifier()
	if err != nil {
		return Source{}, fmt.Errorf("generate source identifier: %w", err)
	}
	now := service.now().UTC()
	source, err := service.store.CreateSource(ctx, Source{
		ID: id, ConnectionID: connection.ID, OwnerUserID: actor.User.ID,
		ProviderFormID: form.ID, Title: form.Title, Module: input.Module,
		State: SourceDraft, SchemaRevision: form.Revision, SchemaFingerprint: form.Fingerprint,
		SyncMode: SyncManual, PollInterval: 15 * time.Minute,
		CreatedAt: now, UpdatedAt: now, Questions: form.Questions,
	})
	if err != nil {
		service.audit(ctx, &actor.User.ID, nil, nil, nil, AuditSourceCreated, auth.AuditOutcomeFailure, nil, requestID)
		return Source{}, err
	}
	service.audit(ctx, &actor.User.ID, nil, &source.ID, nil, AuditSourceCreated, auth.AuditOutcomeSuccess, nil, requestID)
	return source, nil
}

func (service *Service) GetSource(ctx context.Context, actor auth.Session, id Identifier) (Source, error) {
	if err := service.authorize(actor); err != nil {
		return Source{}, err
	}
	return service.store.GetSource(ctx, id, actor.User.ID)
}

func (service *Service) ListSources(ctx context.Context, actor auth.Session, limit, offset int) (SourcePage, error) {
	if err := service.authorize(actor); err != nil {
		return SourcePage{}, err
	}
	if limit == 0 {
		limit = 25
	}
	return service.store.ListSources(ctx, actor.User.ID, limit, offset)
}

func (service *Service) SaveMapping(ctx context.Context, actor auth.Session, id Identifier, version int64, mapping []MappingInput, requestID string) (Source, error) {
	if err := service.authorize(actor); err != nil {
		service.audit(ctx, &actor.User.ID, nil, &id, nil, AuditSourceMapped, auth.AuditOutcomeDenied, nil, requestID)
		return Source{}, err
	}
	source, err := service.store.GetSource(ctx, id, actor.User.ID)
	if err != nil {
		return Source{}, err
	}
	if source.Version != version {
		return Source{}, ErrConflict
	}
	catalog, err := service.moduleCatalog(ctx, actor, source.Module)
	if err != nil {
		return Source{}, err
	}
	operationMapping, err := validateSourceMapping(source, catalog, mapping)
	if err != nil {
		return Source{}, err
	}
	if err := operations.ValidateMapping(catalog, operationMapping); err != nil {
		return Source{}, ErrInvalidInput
	}
	mapped, err := service.store.SaveMapping(ctx, id, actor.User.ID, version, mapping, service.now().UTC())
	if err != nil {
		return Source{}, err
	}
	service.audit(ctx, &actor.User.ID, nil, &id, nil, AuditSourceMapped, auth.AuditOutcomeSuccess, nil, requestID)
	return mapped, nil
}

func (service *Service) UpdateSource(ctx context.Context, actor auth.Session, id Identifier, version int64, input UpdateSourceInput, requestID string) (Source, error) {
	if err := service.authorize(actor); err != nil {
		service.audit(ctx, &actor.User.ID, nil, &id, nil, AuditSourceStateChanged, auth.AuditOutcomeDenied, nil, requestID)
		return Source{}, err
	}
	source, err := service.store.GetSource(ctx, id, actor.User.ID)
	if err != nil {
		return Source{}, err
	}
	if source.Version != version {
		return Source{}, ErrConflict
	}
	if input.Enabled {
		catalog, catalogErr := service.moduleCatalog(ctx, actor, source.Module)
		if catalogErr != nil {
			return Source{}, catalogErr
		}
		mapping := currentSourceMapping(source)
		operationMapping, mappingErr := validateSourceMapping(source, catalog, mapping)
		if mappingErr != nil || operations.ValidateMapping(catalog, operationMapping) != nil {
			return Source{}, ErrInvalidInput
		}
	}
	updated, err := service.store.UpdateSource(ctx, id, actor.User.ID, version, input, service.now().UTC())
	if err != nil {
		return Source{}, err
	}
	service.audit(ctx, &actor.User.ID, nil, &id, nil, AuditSourceStateChanged, auth.AuditOutcomeSuccess, nil, requestID)
	return updated, nil
}

func (service *Service) RefreshSource(ctx context.Context, actor auth.Session, id Identifier, requestID string) (Source, bool, error) {
	if err := service.authorize(actor); err != nil {
		return Source{}, false, err
	}
	source, err := service.store.GetSource(ctx, id, actor.User.ID)
	if err != nil {
		return Source{}, false, err
	}
	connection, err := service.store.GetConnection(ctx, actor.User.ID)
	if err != nil {
		return Source{}, false, err
	}
	refreshToken, err := service.openRefreshToken(connection)
	if err != nil {
		return Source{}, false, err
	}
	form, err := service.provider.GetForm(ctx, refreshToken, source.ProviderFormID)
	if err != nil {
		return Source{}, false, service.handleProviderConnectionError(ctx, actor.User.ID, err)
	}
	updated, drifted, err := service.store.RefreshSourceSchema(ctx, id, actor.User.ID, form, service.now().UTC())
	if err == nil && drifted {
		service.audit(ctx, &actor.User.ID, nil, &id, nil, AuditSourceStateChanged, auth.AuditOutcomeSuccess, nil, requestID)
	}
	return updated, drifted, err
}

func (service *Service) RequestSync(ctx context.Context, actor auth.Session, sourceID Identifier, idempotencyKey, requestID string) (SyncRun, error) {
	if err := service.authorize(actor); err != nil {
		service.audit(ctx, &actor.User.ID, nil, &sourceID, nil, AuditSyncRequested, auth.AuditOutcomeDenied, nil, requestID)
		return SyncRun{}, err
	}
	source, err := service.store.GetSource(ctx, sourceID, actor.User.ID)
	if err != nil {
		return SyncRun{}, err
	}
	if !validIdempotencyKey(idempotencyKey) {
		return SyncRun{}, ErrInvalidInput
	}
	if source.State != SourceActive {
		return SyncRun{}, ErrInvalidState
	}
	now := service.now().UTC()
	id, err := NewIdentifier()
	if err != nil {
		return SyncRun{}, fmt.Errorf("generate sync identifier: %w", err)
	}
	actorID := actor.User.ID
	cursor := syncCursor(source)
	run, err := service.store.CreateSyncRun(ctx, SyncRun{
		ID: id, SourceID: source.ID, OwnerUserID: source.OwnerUserID, ActorUserID: &actorID,
		TriggerKind: TriggerManual, State: SyncQueued, IdempotencyKey: strings.TrimSpace(idempotencyKey),
		CursorStartedAt: cursor, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return SyncRun{}, err
	}
	if run.ID != id || run.RiverJobID > 0 {
		return run, nil
	}
	jobID, err := service.jobs.EnqueueSync(ctx, run.ID)
	if err != nil {
		_, failErr := service.store.FailSync(ctx, run.ID, "enqueue_failed", service.now().UTC())
		return SyncRun{}, errors.Join(err, failErr)
	}
	if err := service.store.SetSyncJob(ctx, run.ID, jobID, now); err != nil {
		cancelErr := service.jobs.Cancel(ctx, jobID)
		_, failErr := service.store.FailSync(ctx, run.ID, "enqueue_failed", service.now().UTC())
		return SyncRun{}, errors.Join(err, cancelErr, failErr)
	}
	run.RiverJobID = jobID
	run.Version++
	service.audit(ctx, &actor.User.ID, nil, nil, &run.ID, AuditSyncRequested, auth.AuditOutcomeSuccess, nil, requestID)
	return run, nil
}

func (service *Service) ListSyncRuns(ctx context.Context, actor auth.Session, sourceID *Identifier, limit, offset int) (SyncRunPage, error) {
	if err := service.authorize(actor); err != nil {
		return SyncRunPage{}, err
	}
	if limit == 0 {
		limit = 25
	}
	return service.store.ListSyncRuns(ctx, actor.User.ID, sourceID, limit, offset)
}

func (service *Service) CancelSync(ctx context.Context, actor auth.Session, id Identifier, version int64, requestID string) (SyncRun, error) {
	if err := service.authorize(actor); err != nil {
		service.audit(ctx, &actor.User.ID, nil, nil, &id, AuditSyncCancelled, auth.AuditOutcomeDenied, nil, requestID)
		return SyncRun{}, err
	}
	cancelled, err := service.store.CancelSync(ctx, id, actor.User.ID, version, service.now().UTC())
	if err != nil {
		return SyncRun{}, err
	}
	if cancelled.RiverJobID > 0 {
		_ = service.jobs.Cancel(ctx, cancelled.RiverJobID)
	}
	service.audit(ctx, &actor.User.ID, nil, nil, &id, AuditSyncCancelled, auth.AuditOutcomeSuccess, nil, requestID)
	return cancelled, nil
}

func (service *Service) authorize(actor auth.Session) error {
	if service == nil || !service.enabled {
		return ErrDisabled
	}
	if !actor.User.Active || !actor.User.Role.CanManageGoogleForms() || actor.User.ID == (auth.Identifier{}) || actor.ID == (auth.Identifier{}) {
		return ErrForbidden
	}
	return nil
}

func (service *Service) moduleCatalog(ctx context.Context, actor auth.Session, module operations.Module) (operations.ModuleCatalog, error) {
	catalog, err := service.operations.Catalog(ctx, actor)
	if err != nil {
		return operations.ModuleCatalog{}, err
	}
	for _, value := range catalog {
		if value.ID == module {
			return value, nil
		}
	}
	return operations.ModuleCatalog{}, ErrForbidden
}

func (service *Service) openRefreshToken(connection Connection) (string, error) {
	if connection.State != ConnectionActive {
		return "", ErrNeedsReauth
	}
	plaintext, err := service.cipher.Open(Ciphertext{
		Data: connection.RefreshTokenCiphertext, Nonce: connection.RefreshTokenNonce,
		KeyVersion: connection.TokenKeyVersion,
	}, refreshTokenAAD(connection.OwnerUserID))
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func (service *Service) handleProviderConnectionError(ctx context.Context, ownerID auth.Identifier, err error) error {
	if errors.Is(err, ErrNeedsReauth) {
		markErr := service.store.MarkConnectionNeedsReauth(ctx, ownerID, "authorization_failed", service.now().UTC())
		return errors.Join(err, markErr)
	}
	return err
}

func (service *Service) audit(ctx context.Context, actorID *auth.Identifier, connectionID, sourceID, syncID *Identifier, eventType AuditEventType, outcome auth.AuditOutcome, affected *int, requestID string) {
	id, err := NewIdentifier()
	if err != nil {
		return
	}
	event := AuditEvent{ID: id, ActorUserID: actorID, ConnectionID: connectionID, SourceID: sourceID,
		SyncRunID: syncID, EventType: eventType, Outcome: outcome, AffectedCount: affected,
		RequestID: truncateText(requestID, 128), CreatedAt: service.now().UTC()}
	if err := service.store.RecordAuditEvent(ctx, event); err != nil && service.onAuditFailure != nil {
		service.onAuditFailure(ctx, event, err)
	}
}

func randomToken(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func oauthVerifierAAD(hash [32]byte) string {
	return "google-forms:oauth-verifier:" + base64.RawURLEncoding.EncodeToString(hash[:])
}

func refreshTokenAAD(ownerID auth.Identifier) string {
	return "google-forms:refresh-token:" + operations.Identifier(ownerID).String()
}

func safeReturnPath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "/cadastro?mode=forms", nil
	}
	if len(value) > 500 || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return "", ErrInvalidInput
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.Fragment != "" {
		return "", ErrInvalidInput
	}
	if parsed.Path == "/admin" {
		query := parsed.Query()
		for key, values := range query {
			if len(values) != 1 || values[0] == "" {
				return "", ErrInvalidInput
			}
			if key != "tab" || values[0] != "integrations" {
				return "", ErrInvalidInput
			}
		}
		return "/admin?tab=integrations", nil
	}
	switch parsed.Path {
	case "/cadastro", "/forms", "/google-forms":
	default:
		return "", ErrInvalidInput
	}
	query := parsed.Query()
	for key, values := range query {
		if len(values) != 1 || values[0] == "" {
			return "", ErrInvalidInput
		}
		switch key {
		case "mode", "tab", "source", "table":
		default:
			return "", ErrInvalidInput
		}
	}
	if mode := query.Get("mode"); mode != "" && mode != "forms" {
		return "", ErrInvalidInput
	}
	tab := query.Get("tab")
	if tab != "" && tab != "sources" && tab != "history" {
		return "", ErrInvalidInput
	}
	source := query.Get("source")
	if source != "" {
		if _, err := ParseIdentifier(source); err != nil {
			return "", ErrInvalidInput
		}
	}
	table := query.Get("table")
	if table != "" && table != "people" && table != "documents" && table != "bills" {
		return "", ErrInvalidInput
	}
	out := url.Values{}
	out.Set("mode", "forms")
	if tab != "" {
		out.Set("tab", tab)
	}
	if source != "" {
		out.Set("source", source)
	}
	if table != "" && table != "people" {
		out.Set("table", table)
	}
	return "/cadastro?" + out.Encode(), nil
}

func providerFormID(reference string) (string, error) {
	reference = strings.TrimSpace(reference)
	if validProviderFormID(reference) {
		return reference, nil
	}
	parsed, err := url.Parse(reference)
	if err != nil || parsed.Scheme != "https" || (parsed.Host != "docs.google.com" && parsed.Host != "forms.google.com") {
		return "", ErrInvalidInput
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for index, part := range parts {
		if part == "d" && index+1 < len(parts) && validProviderFormID(parts[index+1]) {
			return parts[index+1], nil
		}
	}
	return "", ErrInvalidInput
}

func supportedQuestionCount(questions []Question) int {
	count := 0
	for _, question := range questions {
		if question.Supported {
			count++
		}
	}
	return count
}

func validateSourceMapping(source Source, catalog operations.ModuleCatalog, mapping []MappingInput) ([]operations.MappingInput, error) {
	questions := make(map[string]Question, len(source.Questions))
	for _, question := range source.Questions {
		questions[question.ID] = question
	}
	seenQuestions := make(map[string]struct{}, len(mapping))
	seenTargets := make(map[string]struct{}, len(mapping))
	result := make([]operations.MappingInput, 0, len(mapping))
	for _, item := range mapping {
		question, exists := questions[item.QuestionID]
		if !exists || !question.Supported || strings.TrimSpace(item.TargetField) == "" {
			return nil, ErrInvalidInput
		}
		if _, duplicate := seenQuestions[item.QuestionID]; duplicate {
			return nil, ErrInvalidInput
		}
		if _, duplicate := seenTargets[item.TargetField]; duplicate {
			return nil, ErrInvalidInput
		}
		seenQuestions[item.QuestionID] = struct{}{}
		seenTargets[item.TargetField] = struct{}{}
		result = append(result, operations.MappingInput{SourceColumn: question.Position, TargetField: item.TargetField})
	}
	sort.Slice(result, func(left, right int) bool { return result[left].SourceColumn < result[right].SourceColumn })
	return result, nil
}

func currentSourceMapping(source Source) []MappingInput {
	result := make([]MappingInput, 0, len(source.Questions))
	for _, question := range source.Questions {
		if question.TargetField != "" {
			result = append(result, MappingInput{QuestionID: question.ID, TargetField: question.TargetField})
		}
	}
	return result
}

func validIdempotencyKey(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) < 8 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if character < 0x21 || character > 0x7e {
			return false
		}
	}
	return true
}

func overlapCursor(cursor *time.Time) *time.Time {
	if cursor == nil {
		value := time.Unix(0, 0).UTC()
		return &value
	}
	value := cursor.UTC().Add(-CursorOverlap)
	return &value
}

func syncCursor(source Source) *time.Time {
	if source.ResponsePageToken != "" && source.PageTokenCursor != nil {
		value := source.PageTokenCursor.UTC()
		return &value
	}
	return overlapCursor(source.CursorSubmittedAt)
}
