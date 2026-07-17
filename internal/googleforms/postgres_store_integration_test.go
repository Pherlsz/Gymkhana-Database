//go:build integration

package googleforms

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/attachment"
	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/operations"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresGoogleFormsOAuthPaginationDriftStagingAndOwnership(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for Google Forms PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("pgxpool.New() error = %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}

	actorID, _ := auth.NewIdentifier()
	otherID, _ := auth.NewIdentifier()
	memberID, _ := auth.NewIdentifier()
	sessionID, _ := auth.NewIdentifier()
	otherSessionID, _ := auth.NewIdentifier()
	otherUserSessionID, _ := auth.NewIdentifier()
	memberSessionID, _ := auth.NewIdentifier()
	key := "google_forms_" + strings.ReplaceAll(actorID.String(), "-", "")[:16]
	githubID := time.Now().UnixNano()
	if githubID < 0 {
		githubID = -githubID
	}
	if githubID < 10 {
		githubID = 10
	}
	insertGoogleFormsActor(t, ctx, pool, actorID, githubID, key+"_owner", auth.RoleAdmin)
	insertGoogleFormsActor(t, ctx, pool, otherID, githubID+1, key+"_other", auth.RoleAdmin)
	insertGoogleFormsActor(t, ctx, pool, memberID, githubID+2, key+"_member", auth.RoleMember)
	insertGoogleFormsSession(t, ctx, pool, sessionID, actorID, 1)
	insertGoogleFormsSession(t, ctx, pool, otherSessionID, actorID, 2)
	insertGoogleFormsSession(t, ctx, pool, otherUserSessionID, otherID, 3)
	insertGoogleFormsSession(t, ctx, pool, memberSessionID, memberID, 4)

	defer func() {
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, `DELETE FROM google_forms_audit_events
 WHERE actor_user_id=ANY($1::uuid[]) OR source_id IN (SELECT id FROM google_forms_sources WHERE owner_user_id=ANY($1::uuid[]))`, []string{actorID.String(), otherID.String(), memberID.String()})
		_, _ = pool.Exec(cleanup, `DELETE FROM google_forms_response_receipts
 WHERE source_id IN (SELECT id FROM google_forms_sources WHERE owner_user_id=ANY($1::uuid[]))`, []string{actorID.String(), otherID.String(), memberID.String()})
		_, _ = pool.Exec(cleanup, `DELETE FROM google_forms_sync_runs WHERE owner_user_id=ANY($1::uuid[])`, []string{actorID.String(), otherID.String(), memberID.String()})
		_, _ = pool.Exec(cleanup, `DELETE FROM google_forms_sources WHERE owner_user_id=ANY($1::uuid[])`, []string{actorID.String(), otherID.String(), memberID.String()})
		_, _ = pool.Exec(cleanup, `DELETE FROM google_forms_connections WHERE owner_user_id=ANY($1::uuid[])`, []string{actorID.String(), otherID.String(), memberID.String()})
		_, _ = pool.Exec(cleanup, `DELETE FROM google_forms_oauth_states WHERE owner_user_id=ANY($1::uuid[])`, []string{actorID.String(), otherID.String(), memberID.String()})
		_, _ = pool.Exec(cleanup, `DELETE FROM operation_audit_events WHERE actor_user_id=ANY($1::uuid[])`, []string{actorID.String(), otherID.String(), memberID.String()})
		_, _ = pool.Exec(cleanup, `DELETE FROM operation_imports WHERE actor_user_id=ANY($1::uuid[])`, []string{actorID.String(), otherID.String(), memberID.String()})
		_, _ = pool.Exec(cleanup, `DELETE FROM operation_rate_limits WHERE actor_user_id=ANY($1::uuid[])`, []string{actorID.String(), otherID.String(), memberID.String()})
		_, _ = pool.Exec(cleanup, `DELETE FROM profiles WHERE full_name LIKE $1`, key+"%")
		_, _ = pool.Exec(cleanup, `DELETE FROM app_sessions WHERE user_id=ANY($1::uuid[])`, []string{actorID.String(), otherID.String(), memberID.String()})
		_, _ = pool.Exec(cleanup, `DELETE FROM app_users WHERE id=ANY($1::uuid[])`, []string{actorID.String(), otherID.String(), memberID.String()})
	}()

	clock := time.Date(2026, time.July, 17, 12, 0, 0, 0, time.UTC)
	operationJobs := &integrationOperationJobs{}
	operationService, err := operations.NewService(
		operations.NewPostgresStore(pool), integrationObjectStore{}, operationJobs,
		operations.ServiceOptions{Now: func() time.Time { return clock }, RateLimit: 100, MaximumActive: 20},
	)
	if err != nil {
		t.Fatalf("operations.NewService() error = %v", err)
	}
	responses := integrationResponses(key, 12, clock.Add(time.Hour))
	provider := &integrationProvider{form: integrationForm(false), scopes: append([]string(nil), RequiredScopes...), responses: responses}
	googleJobs := &integrationGoogleFormsJobs{}
	cipher, err := NewTokenCipher(1, map[uint16][32]byte{1: {1, 2, 3, 4}})
	if err != nil {
		t.Fatalf("NewTokenCipher() error = %v", err)
	}
	service, err := NewService(NewPostgresStore(pool), provider, cipher, googleJobs, operationService, ServiceOptions{
		Enabled: true, Now: func() time.Time { return clock }, ResponsePageSize: 1, SyncBatchSize: 25,
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	actor := auth.Session{ID: sessionID, User: auth.User{ID: actorID, Role: auth.RoleAdmin, Active: true}}
	wrongSession := actor
	wrongSession.ID = otherSessionID
	other := auth.Session{ID: otherUserSessionID, User: auth.User{ID: otherID, Role: auth.RoleAdmin, Active: true}}
	member := auth.Session{ID: memberSessionID, User: auth.User{ID: memberID, Role: auth.RoleMember, Active: true}}

	if _, err := service.BeginOAuth(ctx, member, "/google-forms", "member-denied"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("member BeginOAuth() error = %v", err)
	}
	inactive := actor
	inactive.User.Active = false
	if _, err := service.BeginOAuth(ctx, inactive, "/google-forms", "inactive-denied"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("inactive BeginOAuth() error = %v", err)
	}

	expiring, err := service.BeginOAuth(ctx, actor, "/google-forms?tab=sources", "oauth-expiring")
	if err != nil {
		t.Fatalf("BeginOAuth(expiring) error = %v", err)
	}
	expiringState := authorizationState(t, expiring.AuthorizationURL)
	clock = clock.Add(OAuthStateTTL + time.Second)
	if _, _, err := service.CompleteOAuth(ctx, actor, expiringState, "expired-code", "oauth-expired"); !errors.Is(err, ErrOAuthState) {
		t.Fatalf("CompleteOAuth(expired) error = %v", err)
	}

	provider.setScopes([]string{ScopeBodyReadonly})
	denied, err := service.BeginOAuth(ctx, actor, "/google-forms", "oauth-scope-start")
	if err != nil {
		t.Fatalf("BeginOAuth(scope denial) error = %v", err)
	}
	if _, _, err := service.CompleteOAuth(ctx, actor, authorizationState(t, denied.AuthorizationURL), "scope-code", "oauth-scope-denied"); !errors.Is(err, ErrOAuthScopes) {
		t.Fatalf("CompleteOAuth(scope denial) error = %v", err)
	}
	if provider.revokeCount() != 1 {
		t.Fatalf("scope denial revoke count = %d", provider.revokeCount())
	}

	provider.setScopes(RequiredScopes)
	started, err := service.BeginOAuth(ctx, actor, "/google-forms?tab=sources", "oauth-start")
	if err != nil {
		t.Fatalf("BeginOAuth() error = %v", err)
	}
	state := authorizationState(t, started.AuthorizationURL)
	if _, _, err := service.CompleteOAuth(ctx, wrongSession, state, "wrong-session", "oauth-wrong-session"); !errors.Is(err, ErrOAuthState) {
		t.Fatalf("CompleteOAuth(wrong session) error = %v", err)
	}
	connection, returnPath, err := service.CompleteOAuth(ctx, actor, state, "valid-code", "oauth-complete")
	if err != nil || returnPath != "/google-forms?tab=sources" || connection.State != ConnectionActive {
		t.Fatalf("CompleteOAuth() = %#v, %q, %v", connection, returnPath, err)
	}
	if _, _, err := service.CompleteOAuth(ctx, actor, state, "replay-code", "oauth-replay"); !errors.Is(err, ErrOAuthState) {
		t.Fatalf("CompleteOAuth(replay) error = %v", err)
	}
	if _, err := service.GetConnection(ctx, other); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner GetConnection() error = %v", err)
	}
	var ciphertext []byte
	if err := pool.QueryRow(ctx, `SELECT refresh_token_ciphertext FROM google_forms_connections WHERE owner_user_id=$1`, actorID.String()).Scan(&ciphertext); err != nil {
		t.Fatalf("load encrypted refresh token: %v", err)
	}
	if bytes.Contains(ciphertext, []byte("refresh-valid-code")) {
		t.Fatal("refresh token was persisted in plaintext")
	}

	provider.setRevokeError(&ProviderError{Code: "provider_unavailable", Retryable: true})
	disconnected, err := service.Disconnect(ctx, actor, connection.Version, "disconnect-best-effort")
	if err != nil || disconnected.State != ConnectionDisconnected {
		t.Fatalf("Disconnect(best effort) = %#v, %v", disconnected, err)
	}
	provider.setRevokeError(nil)
	reconnectedStart, err := service.BeginOAuth(ctx, actor, "/google-forms", "oauth-reconnect-start")
	if err != nil {
		t.Fatalf("BeginOAuth(reconnect) error = %v", err)
	}
	reconnected, _, err := service.CompleteOAuth(ctx, actor, authorizationState(t, reconnectedStart.AuthorizationURL), "reconnect-code", "oauth-reconnect")
	if err != nil || reconnected.ID != connection.ID || reconnected.State != ConnectionActive || reconnected.Version <= connection.Version {
		t.Fatalf("CompleteOAuth(reconnect) = %#v, %v", reconnected, err)
	}

	source, err := service.CreateSource(ctx, actor, CreateSourceInput{
		FormReference: "https://docs.google.com/forms/d/" + provider.form.ID + "/edit", Module: operations.ModuleProfiles,
	}, "source-create")
	if err != nil || len(source.Questions) != 2 || source.Questions[1].UnsupportedCode != "multiple_answers" {
		t.Fatalf("CreateSource() = %#v, %v", source, err)
	}
	if _, err := NewPostgresStore(pool).GetSource(ctx, source.ID, otherID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner GetSource() error = %v", err)
	}
	source, err = service.SaveMapping(ctx, actor, source.ID, source.Version, []MappingInput{{QuestionID: "full-name", TargetField: "full_name"}}, "source-map")
	if err != nil {
		t.Fatalf("SaveMapping() error = %v", err)
	}
	source, err = service.UpdateSource(ctx, actor, source.ID, source.Version, UpdateSourceInput{Enabled: true, SyncMode: SyncManual, PollInterval: 5 * time.Minute}, "source-enable")
	if err != nil || source.State != SourceActive {
		t.Fatalf("UpdateSource(enable) = %#v, %v", source, err)
	}

	provider.setTransientFailures(1)
	first, err := service.RequestSync(ctx, actor, source.ID, "manual-initial-page", "sync-first")
	if err != nil {
		t.Fatalf("RequestSync(first) error = %v", err)
	}
	replayed, err := service.RequestSync(ctx, actor, source.ID, "manual-initial-page", "sync-first-replay")
	if err != nil || replayed.ID != first.ID || googleJobs.syncCount() != 1 {
		t.Fatalf("RequestSync(replay) = %#v, jobs=%d, error=%v", replayed, googleJobs.syncCount(), err)
	}
	if _, err := service.RequestSync(ctx, actor, source.ID, "manual-concurrent-page", "sync-concurrent"); !errors.Is(err, ErrConflict) {
		t.Fatalf("RequestSync(concurrent) error = %v", err)
	}
	if err := service.Sync(ctx, first.ID); err != nil {
		t.Fatalf("Sync(first page window) error = %v", err)
	}
	firstRun := googleFormsRun(t, ctx, service, actor, first.ID)
	if firstRun.State != SyncCompleted || firstRun.StagedCount != MaximumPagesPerRun || firstRun.OperationImportID == nil || provider.listCallCount() < MaximumPagesPerRun+1 {
		t.Fatalf("first sync run = %#v, provider calls=%d", firstRun, provider.listCallCount())
	}
	pending, err := service.ListSources(ctx, actor, 25, 0)
	if err != nil || len(pending.Sources) != 1 || pending.Sources[0].ResponsePageToken == "" || pending.Sources[0].PageTokenCursor == nil {
		t.Fatalf("source continuation = %#v, error=%v", pending, err)
	}
	continuationCursor := *pending.Sources[0].PageTokenCursor
	pendingToken := pending.Sources[0].ResponsePageToken
	if err := NewPostgresStore(pool).MarkConnectionNeedsReauth(ctx, actorID, "authorization_failed", clock); err != nil {
		t.Fatalf("MarkConnectionNeedsReauth(pending page) error = %v", err)
	}
	pendingReconnect, err := service.BeginOAuth(ctx, actor, "/google-forms?tab=sources&source="+source.ID.String(), "oauth-pending-reconnect-start")
	if err != nil {
		t.Fatalf("BeginOAuth(pending page reconnect) error = %v", err)
	}
	if _, _, err := service.CompleteOAuth(ctx, actor, authorizationState(t, pendingReconnect.AuthorizationURL), "pending-reconnect-code", "oauth-pending-reconnect"); err != nil {
		t.Fatalf("CompleteOAuth(pending page reconnect) error = %v", err)
	}
	restored, err := service.ListSources(ctx, actor, 25, 0)
	if err != nil || restored.Sources[0].State != SourcePaused || restored.Sources[0].ResponsePageToken != pendingToken || restored.Sources[0].PageTokenCursor == nil {
		t.Fatalf("reauthorized source lost page continuation: %#v, error=%v", restored, err)
	}
	source = restored.Sources[0]
	source, err = service.UpdateSource(ctx, actor, source.ID, source.Version, UpdateSourceInput{Enabled: true, SyncMode: SyncManual, PollInterval: 5 * time.Minute}, "source-reenable-after-reauth")
	if err != nil {
		t.Fatalf("UpdateSource(after reauth) error = %v", err)
	}

	provider.setForm(integrationForm(true))
	clock = clock.Add(5 * time.Minute)
	queued, err := service.EnqueueDueSources(ctx)
	if err != nil || queued != 1 {
		t.Fatalf("EnqueueDueSources(drift) = %d, %v", queued, err)
	}
	if duplicateTick, err := service.EnqueueDueSources(ctx); err != nil || duplicateTick != 0 {
		t.Fatalf("EnqueueDueSources(duplicate tick) = %d, %v", duplicateTick, err)
	}
	driftRunID := googleJobs.lastSyncID()
	if err := service.Sync(ctx, driftRunID); !errors.Is(err, ErrSchemaDrift) {
		t.Fatalf("Sync(schema drift) error = %v", err)
	}
	drifted, err := service.ListSources(ctx, actor, 25, 0)
	if err != nil || drifted.Sources[0].State != SourceSchemaDrift || drifted.Sources[0].ResponsePageToken != "" ||
		drifted.Sources[0].CursorSubmittedAt == nil || !drifted.Sources[0].CursorSubmittedAt.Equal(continuationCursor) {
		t.Fatalf("drifted source did not roll back its page cursor: %#v, error=%v", drifted, err)
	}
	source = drifted.Sources[0]
	source, err = service.SaveMapping(ctx, actor, source.ID, source.Version, []MappingInput{{QuestionID: "full-name", TargetField: "full_name"}}, "source-remap")
	if err != nil {
		t.Fatalf("SaveMapping(after drift) error = %v", err)
	}
	source, err = service.UpdateSource(ctx, actor, source.ID, source.Version, UpdateSourceInput{Enabled: true, SyncMode: SyncManual, PollInterval: 5 * time.Minute}, "source-reenable")
	if err != nil {
		t.Fatalf("UpdateSource(after drift) error = %v", err)
	}

	recovery, err := service.RequestSync(ctx, actor, source.ID, "manual-recovery-page", "sync-recovery")
	if err != nil {
		t.Fatalf("RequestSync(recovery) error = %v", err)
	}
	if err := service.Sync(ctx, recovery.ID); err != nil {
		t.Fatalf("Sync(recovery overlap) error = %v", err)
	}
	recoveryRun := googleFormsRun(t, ctx, service, actor, recovery.ID)
	if recoveryRun.StagedCount != 0 || recoveryRun.DuplicateCount != MaximumPagesPerRun || recoveryRun.OperationImportID != nil {
		t.Fatalf("recovery overlap run = %#v", recoveryRun)
	}
	clock = clock.Add(5 * time.Minute)
	queued, err = service.EnqueueDueSources(ctx)
	if err != nil || queued != 1 {
		t.Fatalf("EnqueueDueSources(continuation) = %d, %v", queued, err)
	}
	continuationRunID := googleJobs.lastSyncID()
	if err := service.Sync(ctx, continuationRunID); err != nil {
		t.Fatalf("Sync(continuation) error = %v", err)
	}
	continuationRun := googleFormsRun(t, ctx, service, actor, continuationRunID)
	if continuationRun.StagedCount != 2 || continuationRun.OperationImportID == nil {
		t.Fatalf("continuation run = %#v", continuationRun)
	}
	var receiptCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM google_forms_response_receipts WHERE source_id=$1`, source.ID.String()).Scan(&receiptCount); err != nil || receiptCount != 12 {
		t.Fatalf("response receipt count = %d, error=%v", receiptCount, err)
	}

	firstImport, err := operationService.GetImport(ctx, actor, *firstRun.OperationImportID)
	if err != nil || firstImport.SourceKind != operations.SourceGoogleForms || firstImport.ObjectKey != "" || firstImport.OriginalFilename != "" || firstImport.State != operations.ImportReady {
		t.Fatalf("source-neutral Operations import = %#v, error=%v", firstImport, err)
	}
	firstImport, err = operationService.Execute(ctx, actor, firstImport.ID, firstImport.Version, "execute-google-forms")
	if err != nil || firstImport.State != operations.ImportQueued {
		t.Fatalf("Operations Execute() = %#v, %v", firstImport, err)
	}
	if err := operationService.ExecuteImport(ctx, firstImport.ID); err != nil {
		t.Fatalf("Operations ExecuteImport() error = %v", err)
	}
	report, err := operationService.GetReport(ctx, actor, firstImport.ID)
	if err != nil || report.State != operations.ImportCompleted || report.Inserted != MaximumPagesPerRun {
		t.Fatalf("Google Forms Operations report = %#v, error=%v", report, err)
	}
	if _, err := operationService.GetReport(ctx, other, firstImport.ID); !errors.Is(err, operations.ErrNotFound) {
		t.Fatalf("cross-owner Operations report error = %v", err)
	}
	var profileCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM profiles WHERE full_name LIKE $1`, key+" response %").Scan(&profileCount); err != nil || profileCount != MaximumPagesPerRun {
		t.Fatalf("canonical profile count = %d, error=%v", profileCount, err)
	}

	quotaOne := createActiveQuotaSource(t, ctx, service, provider, actor, "quota_form_identifier_1")
	quotaTwo := createActiveQuotaSource(t, ctx, service, provider, actor, "quota_form_identifier_2")
	provider.setForm(integrationForm(true))
	activeOne, err := service.RequestSync(ctx, actor, source.ID, "manual-owner-quota-one", "quota-one")
	if err != nil {
		t.Fatalf("RequestSync(owner quota one) error = %v", err)
	}
	activeTwo, err := service.RequestSync(ctx, actor, quotaOne.ID, "manual-owner-quota-two", "quota-two")
	if err != nil {
		t.Fatalf("RequestSync(owner quota two) error = %v", err)
	}
	if _, err := service.RequestSync(ctx, actor, quotaTwo.ID, "manual-owner-quota-three", "quota-three"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("RequestSync(owner quota three) error = %v", err)
	}
	if _, err := service.CancelSync(ctx, actor, activeOne.ID, activeOne.Version, "quota-one-cancel"); err != nil {
		t.Fatalf("CancelSync(owner quota one) error = %v", err)
	}
	if _, err := service.CancelSync(ctx, actor, activeTwo.ID, activeTwo.Version, "quota-two-cancel"); err != nil {
		t.Fatalf("CancelSync(owner quota two) error = %v", err)
	}

	changedResponses := integrationResponses(key, 12, clock.Add(50*time.Minute))
	changedResponses[0].Answers["full-name"] = []string{key + " response changed"}
	provider.setResponses(changedResponses)
	changedRun, err := service.RequestSync(ctx, actor, source.ID, "manual-response-change", "sync-response-change")
	if err != nil {
		t.Fatalf("RequestSync(response change) error = %v", err)
	}
	if err := service.Sync(ctx, changedRun.ID); !errors.Is(err, ErrResponseChanged) {
		t.Fatalf("Sync(response change) error = %v", err)
	}
	if failed := googleFormsRun(t, ctx, service, actor, changedRun.ID); failed.State != SyncFailed || failed.ErrorCode != "response_changed" {
		t.Fatalf("changed-response sync run = %#v", failed)
	}
	provider.setResponses(responses)

	inactiveRun, err := service.RequestSync(ctx, actor, source.ID, "manual-inactive-actor", "sync-inactive-request")
	if err != nil {
		t.Fatalf("RequestSync(inactive actor) error = %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE app_users SET active=false WHERE id=$1`, actorID.String()); err != nil {
		t.Fatalf("deactivate sync actor: %v", err)
	}
	if err := service.Sync(ctx, inactiveRun.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Sync(inactive actor) error = %v", err)
	}
	if failed := googleFormsRun(t, ctx, service, actor, inactiveRun.ID); failed.State != SyncFailed {
		t.Fatalf("inactive-actor sync run = %#v", failed)
	}
	if _, err := pool.Exec(ctx, `UPDATE app_users SET active=true WHERE id=$1`, actorID.String()); err != nil {
		t.Fatalf("reactivate sync actor: %v", err)
	}

	cancelledRun, err := service.RequestSync(ctx, actor, source.ID, "manual-cancelled-run", "sync-cancel-request")
	if err != nil {
		t.Fatalf("RequestSync(cancel) error = %v", err)
	}
	cancelledRun, err = service.CancelSync(ctx, actor, cancelledRun.ID, cancelledRun.Version, "sync-cancel")
	if err != nil || cancelledRun.State != SyncCancelled || !googleJobs.wasCancelled(cancelledRun.RiverJobID) {
		t.Fatalf("CancelSync() = %#v, error=%v", cancelledRun, err)
	}
	if err := service.Sync(ctx, cancelledRun.ID); !errors.Is(err, ErrCancelled) {
		t.Fatalf("Sync(cancelled) error = %v", err)
	}

	provider.setFormError(ErrNeedsReauth)
	reauthRun, err := service.RequestSync(ctx, actor, source.ID, "manual-invalid-grant", "sync-reauth-request")
	if err != nil {
		t.Fatalf("RequestSync(reauth) error = %v", err)
	}
	if err := service.Sync(ctx, reauthRun.ID); !errors.Is(err, ErrNeedsReauth) {
		t.Fatalf("Sync(invalid grant) error = %v", err)
	}
	connectionAfter, err := service.GetConnection(ctx, actor)
	if err != nil || connectionAfter.State != ConnectionNeedsReauth || len(connectionAfter.RefreshTokenCiphertext) != 0 {
		t.Fatalf("connection after invalid grant = %#v, error=%v", connectionAfter, err)
	}
	failedRun := googleFormsRun(t, ctx, service, actor, reauthRun.ID)
	if failedRun.State != SyncFailed || failedRun.ErrorCode != "needs_reauth" {
		t.Fatalf("invalid-grant sync run = %#v", failedRun)
	}

	var leaked bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (
 SELECT 1 FROM google_forms_audit_events
  WHERE actor_user_id=$1 AND request_id LIKE '%refresh-%'
)`, actorID.String()).Scan(&leaked); err != nil || leaked {
		t.Fatalf("audit secret leak check = %t, error=%v", leaked, err)
	}
}

type integrationProvider struct {
	mu                sync.Mutex
	form              Form
	responses         []Response
	scopes            []string
	revokeErr         error
	formErr           error
	transientFailures int
	revokes           int
	listCalls         int
}

func (provider *integrationProvider) AuthorizationURL(state, _ string) (string, error) {
	return "https://accounts.example.test/authorize?state=" + url.QueryEscape(state), nil
}

func (provider *integrationProvider) Exchange(_ context.Context, code, _ string) (OAuthToken, error) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	return OAuthToken{RefreshToken: "refresh-" + code, Scopes: append([]string(nil), provider.scopes...)}, nil
}

func (provider *integrationProvider) Revoke(_ context.Context, _ string) error {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.revokes++
	return provider.revokeErr
}

func (provider *integrationProvider) GetForm(_ context.Context, _, _ string) (Form, error) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	if provider.formErr != nil {
		return Form{}, provider.formErr
	}
	return provider.form, nil
}

func (provider *integrationProvider) ListResponses(_ context.Context, _, _ string, after *time.Time, pageToken string, pageSize int) (ResponsePage, error) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.listCalls++
	if provider.transientFailures > 0 {
		provider.transientFailures--
		return ResponsePage{}, &ProviderError{Code: "provider_unavailable", Retryable: true}
	}
	start := 0
	if pageToken != "" {
		if !strings.HasPrefix(pageToken, "page:") {
			return ResponsePage{}, &ProviderError{Code: "invalid_page_token"}
		}
		parsed, err := strconv.Atoi(strings.TrimPrefix(pageToken, "page:"))
		if err != nil || parsed < 0 || parsed > len(provider.responses) {
			return ResponsePage{}, &ProviderError{Code: "invalid_page_token"}
		}
		start = parsed
	}
	filtered := make([]Response, 0, len(provider.responses))
	for _, response := range provider.responses {
		if after == nil || !response.SubmittedAt.Before(*after) {
			filtered = append(filtered, response)
		}
	}
	if start > len(filtered) {
		return ResponsePage{}, &ProviderError{Code: "invalid_page_token"}
	}
	end := min(start+pageSize, len(filtered))
	page := ResponsePage{Responses: append([]Response(nil), filtered[start:end]...)}
	if end < len(filtered) {
		page.NextPageToken = "page:" + strconv.Itoa(end)
	}
	return page, nil
}

func (provider *integrationProvider) setScopes(scopes []string) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.scopes = append([]string(nil), scopes...)
}

func (provider *integrationProvider) setRevokeError(err error) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.revokeErr = err
}

func (provider *integrationProvider) setForm(form Form) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.form = form
}

func (provider *integrationProvider) setFormError(err error) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.formErr = err
}

func (provider *integrationProvider) setTransientFailures(count int) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.transientFailures = count
}

func (provider *integrationProvider) setResponses(responses []Response) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.responses = append([]Response(nil), responses...)
}

func (provider *integrationProvider) revokeCount() int {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	return provider.revokes
}

func (provider *integrationProvider) listCallCount() int {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	return provider.listCalls
}

type integrationGoogleFormsJobs struct {
	mu        sync.Mutex
	next      int64
	syncIDs   []Identifier
	cancelled map[int64]bool
}

func (jobs *integrationGoogleFormsJobs) EnqueueSync(_ context.Context, id Identifier) (int64, error) {
	jobs.mu.Lock()
	defer jobs.mu.Unlock()
	jobs.next++
	jobs.syncIDs = append(jobs.syncIDs, id)
	return jobs.next, nil
}

func (jobs *integrationGoogleFormsJobs) EnqueueDue(_ context.Context, _ time.Time) (int64, error) {
	jobs.mu.Lock()
	defer jobs.mu.Unlock()
	jobs.next++
	return jobs.next, nil
}

func (jobs *integrationGoogleFormsJobs) Cancel(_ context.Context, id int64) error {
	jobs.mu.Lock()
	defer jobs.mu.Unlock()
	if jobs.cancelled == nil {
		jobs.cancelled = make(map[int64]bool)
	}
	jobs.cancelled[id] = true
	return nil
}

func (jobs *integrationGoogleFormsJobs) syncCount() int {
	jobs.mu.Lock()
	defer jobs.mu.Unlock()
	return len(jobs.syncIDs)
}

func (jobs *integrationGoogleFormsJobs) lastSyncID() Identifier {
	jobs.mu.Lock()
	defer jobs.mu.Unlock()
	return jobs.syncIDs[len(jobs.syncIDs)-1]
}

func (jobs *integrationGoogleFormsJobs) wasCancelled(id int64) bool {
	jobs.mu.Lock()
	defer jobs.mu.Unlock()
	return jobs.cancelled[id]
}

type integrationOperationJobs struct {
	mu   sync.Mutex
	next int64
}

func (jobs *integrationOperationJobs) enqueue() (int64, error) {
	jobs.mu.Lock()
	defer jobs.mu.Unlock()
	jobs.next++
	return jobs.next, nil
}

func (jobs *integrationOperationJobs) EnqueueParse(context.Context, operations.Identifier) (int64, error) {
	return jobs.enqueue()
}
func (jobs *integrationOperationJobs) EnqueueExecute(context.Context, operations.Identifier) (int64, error) {
	return jobs.enqueue()
}
func (jobs *integrationOperationJobs) EnqueueExport(context.Context, operations.Identifier) (int64, error) {
	return jobs.enqueue()
}
func (jobs *integrationOperationJobs) EnqueueCleanup(context.Context, time.Time) (int64, error) {
	return jobs.enqueue()
}
func (jobs *integrationOperationJobs) Cancel(context.Context, int64) error { return nil }

type integrationObjectStore struct{}

func (integrationObjectStore) PresignOperationUpload(context.Context, string, int64, time.Duration) (attachment.SignedRequest, error) {
	return attachment.SignedRequest{}, errors.New("unexpected object upload")
}
func (integrationObjectStore) PresignDownload(context.Context, string, string, string, time.Duration) (attachment.SignedRequest, error) {
	return attachment.SignedRequest{}, errors.New("unexpected object download")
}
func (integrationObjectStore) Open(context.Context, string) (io.ReadCloser, error) {
	return nil, errors.New("unexpected object read")
}
func (integrationObjectStore) Put(context.Context, string, io.Reader, int64, string) error {
	return errors.New("unexpected object write")
}
func (integrationObjectStore) Delete(context.Context, string) error { return nil }

func integrationForm(drifted bool) Form {
	questions := []Question{
		{ID: "full-name", Position: 0, Title: "Nome completo", AnswerKind: AnswerText, Required: true, Supported: true},
		{ID: "checkboxes", Position: 1, Title: "Opções", AnswerKind: AnswerUnsupported, Supported: false, UnsupportedCode: "multiple_answers"},
	}
	if drifted {
		questions = append(questions, Question{ID: "city", Position: 2, Title: "Cidade", AnswerKind: AnswerText, Supported: true})
	}
	for index := range questions {
		questions[index].QuestionFingerprint = fingerprintQuestion(questions[index])
	}
	form := Form{ID: "form_identifier_123", Title: "Inscrições", Revision: "revision-1", Questions: questions}
	if drifted {
		form.Revision = "revision-2"
	}
	form.Fingerprint = fingerprintForm(questions)
	return form
}

func integrationResponses(prefix string, count int, submittedAt time.Time) []Response {
	result := make([]Response, 0, count)
	for index := count; index > 0; index-- {
		result = append(result, Response{
			ID: fmt.Sprintf("response-%02d", index), SubmittedAt: submittedAt,
			Answers: map[string][]string{"full-name": {fmt.Sprintf("%s response %02d", prefix, index)}},
		})
	}
	return result
}

func authorizationState(t *testing.T, value string) string {
	t.Helper()
	parsed, err := url.Parse(value)
	if err != nil || parsed.Query().Get("state") == "" {
		t.Fatalf("invalid authorization URL %q: %v", value, err)
	}
	return parsed.Query().Get("state")
}

func googleFormsRun(t *testing.T, ctx context.Context, service *Service, actor auth.Session, id Identifier) SyncRun {
	t.Helper()
	page, err := service.ListSyncRuns(ctx, actor, nil, 100, 0)
	if err != nil {
		t.Fatalf("ListSyncRuns() error = %v", err)
	}
	for _, run := range page.Runs {
		if run.ID == id {
			return run
		}
	}
	t.Fatalf("sync run %s not found", id)
	return SyncRun{}
}

func createActiveQuotaSource(t *testing.T, ctx context.Context, service *Service, provider *integrationProvider, actor auth.Session, formID string) Source {
	t.Helper()
	form := integrationForm(false)
	form.ID = formID
	form.Title = "Fonte de quota " + formID
	provider.setForm(form)
	source, err := service.CreateSource(ctx, actor, CreateSourceInput{FormReference: formID, Module: operations.ModuleProfiles}, "quota-source-create")
	if err != nil {
		t.Fatalf("CreateSource(%s) error = %v", formID, err)
	}
	source, err = service.SaveMapping(ctx, actor, source.ID, source.Version, []MappingInput{{QuestionID: "full-name", TargetField: "full_name"}}, "quota-source-map")
	if err != nil {
		t.Fatalf("SaveMapping(%s) error = %v", formID, err)
	}
	source, err = service.UpdateSource(ctx, actor, source.ID, source.Version, UpdateSourceInput{Enabled: true, SyncMode: SyncManual, PollInterval: 5 * time.Minute}, "quota-source-enable")
	if err != nil {
		t.Fatalf("UpdateSource(%s) error = %v", formID, err)
	}
	return source
}

func insertGoogleFormsActor(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id auth.Identifier, githubID int64, login string, role auth.Role) {
	t.Helper()
	if _, err := pool.Exec(ctx, `INSERT INTO app_users
(id, github_user_id, github_login, display_name, role, active)
VALUES($1,$2,$3,$4,$5,true)`, id.String(), githubID, login, "Google Forms integration actor", role); err != nil {
		t.Fatalf("insert Google Forms actor: %v", err)
	}
}

func insertGoogleFormsSession(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id, userID auth.Identifier, marker byte) {
	t.Helper()
	if _, err := pool.Exec(ctx, `INSERT INTO app_sessions
(id, user_id, token_hash, expires_at)
VALUES($1,$2,$3,$4)`, id.String(), userID.String(), bytes.Repeat([]byte{marker}, 32), time.Now().UTC().Add(time.Hour)); err != nil {
		t.Fatalf("insert Google Forms session: %v", err)
	}
}
