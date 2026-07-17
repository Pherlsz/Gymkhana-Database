package matching

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type failingAnalysisJobInserter struct{}

func (failingAnalysisJobInserter) EnqueueAnalysisTx(context.Context, pgx.Tx, Identifier) (int64, error) {
	return 0, errors.New("queue unavailable")
}

func TestPostgresMatchingEvidenceNoMatchAndDeterministicBounds(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for Matching PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("pgxpool.New() error = %v", err)
	}
	defer pool.Close()
	now := time.Now().UTC().Truncate(time.Second)
	actorID := auth.Identifier(newTestIdentifier())
	caseFirst, caseSecond := newTestIdentifier(), newTestIdentifier()
	fuzzyFirst, fuzzySecond := newTestIdentifier(), newTestIdentifier()
	weakFirst, weakSecond := newTestIdentifier(), newTestIdentifier()
	profileIDs := []Identifier{caseFirst, caseSecond, fuzzyFirst, fuzzySecond, weakFirst, weakSecond}
	for range 8 {
		profileIDs = append(profileIDs, newTestIdentifier())
	}
	key := "matching_vectors_" + caseFirst.String()[0:8]
	githubID := now.UnixNano() + 193
	if githubID < 0 {
		githubID = -githubID
	}
	if _, err := pool.Exec(ctx, `INSERT INTO app_users(id,github_user_id,github_login,display_name,role,active)
VALUES($1,$2,$3,'Matching Vectors','MEMBER',true)`, matchingAuthUUID(actorID), githubID, key); err != nil {
		t.Fatalf("insert vector actor: %v", err)
	}
	defer cleanupMatchingCandidateVectors(t, pool, actorID, profileIDs)
	if _, err := pool.Exec(ctx, `INSERT INTO profiles
(id,full_name,email,address_street,address_city,address_state,address_postal_code) VALUES
($1,'Marcos Lima',$7,NULL,NULL,NULL,NULL),
($2,'MARCOS LIMA',$8,NULL,NULL,NULL,NULL),
($3,'Mariana Oliveira Costa',$9,'Rua das Flores Centro','Recife','PE',NULL),
($4,'Mariana Oliveira Csta',$10,'Rua das Florez Centro','Recife','PE',NULL),
($5,'Bruno Carvalho',$11,NULL,'Curitiba','PR','80000000'),
($6,'Zuleica Nascimento',$12,NULL,'Curitiba','PR','80000000')`,
		matchingUUID(caseFirst), matchingUUID(caseSecond), matchingUUID(fuzzyFirst), matchingUUID(fuzzySecond),
		matchingUUID(weakFirst), matchingUUID(weakSecond), key+"-1@example.org", key+"-2@example.org",
		key+"-3@example.org", key+"-4@example.org", key+"-5@example.org", key+"-6@example.org"); err != nil {
		t.Fatalf("insert evidence Profiles: %v", err)
	}
	store := NewPostgresStore(pool)
	createAndRunMatchingAnalysis(t, ctx, store, actorID, "matching-vector-analysis", now)
	page, err := store.ListCases(ctx, CaseListOptions{States: []CaseState{CasePending}, Limit: 100})
	if err != nil {
		t.Fatalf("ListCases(vectors) error = %v", err)
	}
	caseMatch, found := lookupPair(page.Cases, caseFirst, caseSecond)
	if !found || !hasEvidence(caseMatch.Evidence, EvidenceNameExact) || caseMatch.Score < MediumCandidateScore {
		t.Fatalf("case-insensitive exact-name candidate = %#v, found=%v", caseMatch, found)
	}
	fuzzyMatch, found := lookupPair(page.Cases, fuzzyFirst, fuzzySecond)
	if !found || !hasEvidence(fuzzyMatch.Evidence, EvidenceNameSimilar) || !hasEvidence(fuzzyMatch.Evidence, EvidenceAddressSimilar) {
		t.Fatalf("fuzzy name/address candidate = %#v, found=%v", fuzzyMatch, found)
	}
	if _, found := lookupPair(page.Cases, weakFirst, weakSecond); found {
		t.Fatal("weak city/postal-only evidence unexpectedly created a candidate")
	}
	for index, id := range profileIDs[6:] {
		if _, err := pool.Exec(ctx, `INSERT INTO profiles(id,full_name,email) VALUES($1,$2,$3)`,
			matchingUUID(id), "Bounded Person "+string(rune('A'+index)), key+"-bounded@example.org"); err != nil {
			t.Fatalf("insert bounded Profile %d: %v", index, err)
		}
	}
	firstAnalysis := createAndRunMatchingAnalysisWithMaximum(t, ctx, store, actorID, "matching-bound-0001", now.Add(time.Second), 5)
	secondAnalysis := createAndRunMatchingAnalysisWithMaximum(t, ctx, store, actorID, "matching-bound-0002", now.Add(2*time.Second), 5)
	firstPairs := matchingAnalysisPairs(t, ctx, pool, firstAnalysis)
	secondPairs := matchingAnalysisPairs(t, ctx, pool, secondAnalysis)
	if len(firstPairs) != 5 || !slices.Equal(firstPairs, secondPairs) {
		t.Fatalf("bounded deterministic pairs = %#v / %#v", firstPairs, secondPairs)
	}
}

func TestPostgresMatchingAnalysisConcurrencyIdempotencyAndRate(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for Matching PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("pgxpool.New() error = %v", err)
	}
	defer pool.Close()
	now := time.Now().UTC().Truncate(time.Second)
	actorID := auth.Identifier(newTestIdentifier())
	key := "matching_concurrency_" + newTestIdentifier().String()[0:8]
	githubID := now.UnixNano() + 389
	if githubID < 0 {
		githubID = -githubID
	}
	if _, err := pool.Exec(ctx, `INSERT INTO app_users(id,github_user_id,github_login,display_name,role,active)
VALUES($1,$2,$3,'Matching Concurrency','MEMBER',true)`, matchingAuthUUID(actorID), githubID, key); err != nil {
		t.Fatalf("insert concurrency actor: %v", err)
	}
	defer func() {
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, `DELETE FROM matching_audit_events WHERE actor_user_id=$1`, matchingAuthUUID(actorID))
		_, _ = pool.Exec(cleanup, `DELETE FROM matching_analyses WHERE actor_user_id=$1`, matchingAuthUUID(actorID))
		_, _ = pool.Exec(cleanup, `DELETE FROM matching_rate_limits WHERE actor_user_id=$1`, matchingAuthUUID(actorID))
		_, _ = pool.Exec(cleanup, `DELETE FROM app_users WHERE id=$1`, matchingAuthUUID(actorID))
	}()
	store := NewPostgresStore(pool)
	type creation struct {
		analysis Analysis
		fresh    bool
		err      error
	}
	start := make(chan struct{})
	results := make(chan creation, 2)
	for range 2 {
		go func() {
			<-start
			value, fresh, createErr := store.CreateAnalysis(ctx, CreateAnalysisInput{
				ID: newTestIdentifier(), ActorUserID: actorID, IdempotencyKey: "matching-concurrent-key", ExpiresAt: now.Add(24 * time.Hour),
			}, now.Truncate(AnalysisWindow), 5)
			results <- creation{analysis: value, fresh: fresh, err: createErr}
		}()
	}
	close(start)
	first, second := <-results, <-results
	if first.err != nil || second.err != nil || first.analysis.ID != second.analysis.ID || first.fresh == second.fresh {
		t.Fatalf("concurrent idempotent creations = %#v / %#v", first, second)
	}
	created := first
	if !created.fresh {
		created = second
	}
	if _, _, err := store.CreateAnalysis(ctx, CreateAnalysisInput{
		ID: newTestIdentifier(), ActorUserID: actorID, IdempotencyKey: "matching-other-active", ExpiresAt: now.Add(24 * time.Hour),
	}, now.Truncate(AnalysisWindow), 5); !errors.Is(err, ErrConflict) {
		t.Fatalf("second active analysis error = %v", err)
	}
	if err := store.FailAnalysis(ctx, created.analysis.ID, "test_complete", AnalysisFailed, now.Add(time.Second)); err != nil {
		t.Fatalf("FailAnalysis(concurrency fixture) error = %v", err)
	}
	if _, _, err := store.CreateAnalysis(ctx, CreateAnalysisInput{
		ID: newTestIdentifier(), ActorUserID: actorID, IdempotencyKey: "matching-rate-limit", ExpiresAt: now.Add(24 * time.Hour),
	}, now.Truncate(AnalysisWindow), 1); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("persistent rate limit error = %v", err)
	}
}

func TestPostgresMatchingAnalysisAndRiverJobCommitAtomically(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for Matching PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("pgxpool.New() error = %v", err)
	}
	defer pool.Close()
	now := time.Now().UTC().Truncate(time.Second)
	actorID := auth.Identifier(newTestIdentifier())
	analysisID, rolledBackID := newTestIdentifier(), newTestIdentifier()
	key := "matching_transaction_" + analysisID.String()[0:8]
	githubID := now.UnixNano() + 521
	if githubID < 0 {
		githubID = -githubID
	}
	if _, err := pool.Exec(ctx, `INSERT INTO app_users(id,github_user_id,github_login,display_name,role,active)
VALUES($1,$2,$3,'Matching Transaction','MEMBER',true)`, matchingAuthUUID(actorID), githubID, key); err != nil {
		t.Fatalf("insert transactional actor: %v", err)
	}
	var riverJobID int64
	defer func() {
		cleanup := context.Background()
		if riverJobID > 0 {
			_, _ = pool.Exec(cleanup, `DELETE FROM river_job WHERE id=$1`, riverJobID)
		}
		_, _ = pool.Exec(cleanup, `DELETE FROM matching_analyses WHERE actor_user_id=$1`, matchingAuthUUID(actorID))
		_, _ = pool.Exec(cleanup, `DELETE FROM matching_rate_limits WHERE actor_user_id=$1`, matchingAuthUUID(actorID))
		_, _ = pool.Exec(cleanup, `DELETE FROM app_users WHERE id=$1`, matchingAuthUUID(actorID))
	}()
	jobs := NewRiverJobs()
	client, err := NewRiverClient(pool, nil, false)
	if err != nil {
		t.Fatalf("NewRiverClient() error = %v", err)
	}
	if err := jobs.SetClient(client); err != nil {
		t.Fatalf("SetClient() error = %v", err)
	}
	store := NewPostgresStore(pool)
	analysis, fresh, err := store.CreateAnalysisWithJob(ctx, CreateAnalysisInput{
		ID: analysisID, ActorUserID: actorID, IdempotencyKey: "matching-transactional-job", ExpiresAt: now.Add(24 * time.Hour),
	}, now.Truncate(AnalysisWindow), 5, jobs)
	if err != nil || !fresh || analysis.RiverJobID <= 0 || analysis.State != AnalysisQueued {
		t.Fatalf("CreateAnalysisWithJob() = %#v, fresh=%v, error=%v", analysis, fresh, err)
	}
	riverJobID = analysis.RiverJobID
	var kind, queuedAnalysisID string
	if err := pool.QueryRow(ctx, `SELECT kind,args->>'analysis_id' FROM river_job WHERE id=$1`, riverJobID).Scan(&kind, &queuedAnalysisID); err != nil {
		t.Fatalf("read transactional River job: %v", err)
	}
	if kind != (AnalysisArgs{}).Kind() || queuedAnalysisID != analysisID.String() {
		t.Fatalf("transactional River job kind=%q analysis_id=%q", kind, queuedAnalysisID)
	}
	if err := store.FailAnalysis(ctx, analysisID, "test_complete", AnalysisFailed, now.Add(time.Second)); err != nil {
		t.Fatalf("FailAnalysis(transactional fixture) error = %v", err)
	}
	if _, _, err := store.CreateAnalysisWithJob(ctx, CreateAnalysisInput{
		ID: rolledBackID, ActorUserID: actorID, IdempotencyKey: "matching-transaction-rollback", ExpiresAt: now.Add(24 * time.Hour),
	}, now.Truncate(AnalysisWindow), 5, failingAnalysisJobInserter{}); err == nil {
		t.Fatal("CreateAnalysisWithJob(queue failure) unexpectedly succeeded")
	}
	var rolledBackAnalyses, requestCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM matching_analyses WHERE id=$1`, matchingUUID(rolledBackID)).Scan(&rolledBackAnalyses); err != nil {
		t.Fatalf("count rolled-back analysis: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT request_count FROM matching_rate_limits WHERE actor_user_id=$1`, matchingAuthUUID(actorID)).Scan(&requestCount); err != nil {
		t.Fatalf("read transactional rate count: %v", err)
	}
	if rolledBackAnalyses != 0 || requestCount != 1 {
		t.Fatalf("queue failure rollback: analyses=%d request_count=%d", rolledBackAnalyses, requestCount)
	}
}

func TestPostgresMatchingCancellationAndRetentionPreserveHumanDecision(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for Matching PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("pgxpool.New() error = %v", err)
	}
	defer pool.Close()
	now := time.Now().UTC().Truncate(time.Second)
	actorID := auth.Identifier(newTestIdentifier())
	firstID, secondID := newTestIdentifier(), newTestIdentifier()
	cancelledID, retainedID, caseID := newTestIdentifier(), newTestIdentifier(), newTestIdentifier()
	key := "matching_retention_" + caseID.String()[0:8]
	githubID := now.UnixNano() + 613
	if githubID < 0 {
		githubID = -githubID
	}
	if _, err := pool.Exec(ctx, `INSERT INTO app_users(id,github_user_id,github_login,display_name,role,active)
VALUES($1,$2,$3,'Matching Retention','MEMBER',true)`, matchingAuthUUID(actorID), githubID, key); err != nil {
		t.Fatalf("insert retention actor: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO profiles(id,full_name,email) VALUES
($1,'Retention One',$3),($2,'Retention Two',$4)`, matchingUUID(firstID), matchingUUID(secondID), key+"-1@example.org", key+"-2@example.org"); err != nil {
		t.Fatalf("insert retention Profiles: %v", err)
	}
	defer func() {
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, `DELETE FROM matching_audit_events WHERE case_id=$1 OR actor_user_id=$2`, matchingUUID(caseID), matchingAuthUUID(actorID))
		_, _ = pool.Exec(cleanup, `DELETE FROM matching_case_decisions WHERE case_id=$1`, matchingUUID(caseID))
		_, _ = pool.Exec(cleanup, `DELETE FROM matching_analyses WHERE actor_user_id=$1`, matchingAuthUUID(actorID))
		_, _ = pool.Exec(cleanup, `DELETE FROM matching_cases WHERE id=$1`, matchingUUID(caseID))
		_, _ = pool.Exec(cleanup, `DELETE FROM profiles WHERE id IN ($1,$2)`, matchingUUID(firstID), matchingUUID(secondID))
		_, _ = pool.Exec(cleanup, `DELETE FROM matching_rate_limits WHERE actor_user_id=$1`, matchingAuthUUID(actorID))
		_, _ = pool.Exec(cleanup, `DELETE FROM app_users WHERE id=$1`, matchingAuthUUID(actorID))
	}()
	store := NewPostgresStore(pool)
	cancelled, fresh, err := store.CreateAnalysis(ctx, CreateAnalysisInput{
		ID: cancelledID, ActorUserID: actorID, IdempotencyKey: "matching-cancel-retention", ExpiresAt: now.Add(time.Hour),
	}, now.Truncate(AnalysisWindow), 10)
	if err != nil || !fresh || cancelled.State != AnalysisQueued {
		t.Fatalf("CreateAnalysis(cancellation) = %#v, fresh=%v, error=%v", cancelled, fresh, err)
	}
	cancelled, err = store.RequestAnalysisCancellation(ctx, cancelledID, actorID, now.Add(time.Second))
	if err != nil || cancelled.State != AnalysisCancelled || cancelled.CancelRequestedAt == nil {
		t.Fatalf("RequestAnalysisCancellation() = %#v, error=%v", cancelled, err)
	}
	replayedCancellation, err := store.RequestAnalysisCancellation(ctx, cancelledID, actorID, now.Add(2*time.Second))
	if err != nil || replayedCancellation.Version != cancelled.Version {
		t.Fatalf("RequestAnalysisCancellation(replay) = %#v, error=%v", replayedCancellation, err)
	}
	if _, claimed, err := store.ClaimAnalysis(ctx, cancelledID, now.Add(2*time.Second)); err != nil || claimed {
		t.Fatalf("ClaimAnalysis(cancelled) claimed=%v, error=%v", claimed, err)
	}
	if _, err := store.GenerateCandidates(ctx, cancelledID, 10, AnalysisTimeout, now.Add(2*time.Second)); !errors.Is(err, ErrCancelled) {
		t.Fatalf("GenerateCandidates(cancelled) error = %v", err)
	}
	retained, fresh, err := store.CreateAnalysis(ctx, CreateAnalysisInput{
		ID: retainedID, ActorUserID: actorID, IdempotencyKey: "matching-retained-decision", ExpiresAt: now.Add(time.Hour),
	}, now.Truncate(AnalysisWindow), 10)
	if err != nil || !fresh {
		t.Fatalf("CreateAnalysis(retention) = %#v, fresh=%v, error=%v", retained, fresh, err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO matching_cases
(id,left_profile_id,right_profile_id,left_profile_version,right_profile_version,score,score_band,state,first_analysis_id,last_analysis_id,created_at,updated_at)
VALUES($1,LEAST($2::uuid,$3::uuid),GREATEST($2::uuid,$3::uuid),1,1,90,'HIGH','PENDING',$4,$4,$5,$5)`,
		matchingUUID(caseID), matchingUUID(firstID), matchingUUID(secondID), matchingUUID(retainedID), now); err != nil {
		t.Fatalf("insert retained matching case: %v", err)
	}
	if _, err := store.DismissCase(ctx, caseID, actorID, 1, "retention-dismiss", now.Add(3*time.Second)); err != nil {
		t.Fatalf("DismissCase(retention) error = %v", err)
	}
	if err := store.FailAnalysis(ctx, retainedID, "test_complete", AnalysisFailed, now.Add(4*time.Second)); err != nil {
		t.Fatalf("FailAnalysis(retention) error = %v", err)
	}
	deleted, err := store.CleanupAnalyses(ctx, now.Add(2*time.Hour), 10)
	if err != nil || deleted != 2 {
		t.Fatalf("CleanupAnalyses() deleted=%d, error=%v", deleted, err)
	}
	var caseState CaseState
	var firstAnalysis pgtype.UUID
	var decisions int
	if err := pool.QueryRow(ctx, `SELECT state,first_analysis_id FROM matching_cases WHERE id=$1`, matchingUUID(caseID)).Scan(&caseState, &firstAnalysis); err != nil {
		t.Fatalf("read retained matching case: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM matching_case_decisions WHERE case_id=$1 AND action='NOT_DUPLICATE'`, matchingUUID(caseID)).Scan(&decisions); err != nil {
		t.Fatalf("count retained matching decisions: %v", err)
	}
	if caseState != CaseNotDuplicate || firstAnalysis.Valid || decisions != 1 {
		t.Fatalf("retained human decision state=%s first_analysis=%v decisions=%d", caseState, firstAnalysis.Valid, decisions)
	}
}

func TestPostgresMatchingCandidateReviewAndTransactionalMerge(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for Matching PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("pgxpool.New() error = %v", err)
	}
	defer pool.Close()
	now := time.Now().UTC().Truncate(time.Second)
	actorID := auth.Identifier(newTestIdentifier())
	survivorID, sourceID, unrelatedID := newTestIdentifier(), newTestIdentifier(), newTestIdentifier()
	documentTypeID, documentID := newTestIdentifier(), newTestIdentifier()
	billTypeID, billID := newTestIdentifier(), newTestIdentifier()
	customFieldID, customValueID, survivorCustomValueID := newTestIdentifier(), newTestIdentifier(), newTestIdentifier()
	attachmentFieldID, intentID, attachmentID := newTestIdentifier(), newTestIdentifier(), newTestIdentifier()
	entityTypeID, entityID := newTestIdentifier(), newTestIdentifier()
	key := "matching_" + survivorID.String()[0:8]
	githubID := now.UnixNano()
	if githubID < 0 {
		githubID = -githubID
	}
	if _, err := pool.Exec(ctx, `INSERT INTO app_users(id,github_user_id,github_login,display_name,role,active)
VALUES($1,$2,$3,'Matching Admin','ADMIN',true)`, matchingAuthUUID(actorID), githubID, key); err != nil {
		t.Fatalf("insert matching actor: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO profiles
(id,full_name,email,address_city,address_state,address_postal_code)
VALUES
($1,'Ana Maria Souza',$4,'Porto Alegre','RS','90000000'),
($2,'Ana M Souza',$4,'Porto Alegre','RS','90000000'),
($3,'Carlos Sem Relação',$5,'Recife','PE','50000000')`,
		matchingUUID(survivorID), matchingUUID(sourceID), matchingUUID(unrelatedID), key+"@example.org", key+"-other@example.org"); err != nil {
		t.Fatalf("insert matching Profiles: %v", err)
	}
	defer cleanupMatchingIntegration(t, pool, actorID, []Identifier{survivorID, sourceID, unrelatedID},
		[]Identifier{documentTypeID, billTypeID, customFieldID, attachmentFieldID, entityTypeID})

	store := NewPostgresStore(pool)
	analysisID := createAndRunMatchingAnalysis(t, ctx, store, actorID, "matching-analysis-0001", now)
	analysis, err := store.GetAnalysis(ctx, analysisID, actorID)
	if err != nil || analysis.State != AnalysisCompleted || analysis.ProfilesScanned < 3 || analysis.CandidateCount < 1 {
		t.Fatalf("completed matching analysis = %#v, error=%v", analysis, err)
	}
	page, err := store.ListCases(ctx, CaseListOptions{States: []CaseState{CasePending}, Limit: 100})
	if err != nil {
		t.Fatalf("ListCases() error = %v", err)
	}
	matchingCase := findPair(t, page.Cases, survivorID, sourceID)
	if matchingCase.Score < HighCandidateScore || !hasEvidence(matchingCase.Evidence, EvidenceEmailExact) ||
		matchingCase.Left == nil || matchingCase.Right == nil {
		t.Fatalf("unexpected matching candidate: %#v", matchingCase)
	}
	createAndRunMatchingAnalysis(t, ctx, store, actorID, "matching-analysis-repeat", now.Add(500*time.Millisecond))
	unchanged, err := store.GetCase(ctx, matchingCase.ID)
	if err != nil || unchanged.Version != matchingCase.Version || !unchanged.UpdatedAt.Equal(matchingCase.UpdatedAt) {
		t.Fatalf("unchanged candidate was rewritten: before=%#v after=%#v error=%v", matchingCase, unchanged, err)
	}
	dismissed, err := store.DismissCase(ctx, matchingCase.ID, actorID, matchingCase.Version, "dismiss-request", now.Add(time.Second))
	if err != nil || dismissed.State != CaseNotDuplicate {
		t.Fatalf("DismissCase() = %#v, error=%v", dismissed, err)
	}
	var dismissalAudits int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM matching_audit_events
WHERE case_id=$1 AND event_type='CASE_DISMISSED' AND outcome='SUCCESS'`, matchingUUID(matchingCase.ID)).Scan(&dismissalAudits); err != nil || dismissalAudits != 1 {
		t.Fatalf("atomic dismissal audit count = %d, error=%v", dismissalAudits, err)
	}
	suppressedAnalysisID := createAndRunMatchingAnalysis(t, ctx, store, actorID, "matching-analysis-0002", now.Add(2*time.Second))
	var suppressedLinks int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM matching_analysis_cases
WHERE analysis_id=$1 AND case_id=$2`, matchingUUID(suppressedAnalysisID), matchingUUID(matchingCase.ID)).Scan(&suppressedLinks); err != nil || suppressedLinks != 0 {
		t.Fatalf("unchanged dismissal candidate links = %d, error=%v", suppressedLinks, err)
	}
	suppressed, err := store.GetCase(ctx, matchingCase.ID)
	if err != nil || suppressed.State != CaseNotDuplicate {
		t.Fatalf("unchanged dismissed case = %#v, error=%v", suppressed, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE profiles SET full_name='Ana Maria de Souza',version=version+1,updated_at=$2 WHERE id=$1`, matchingUUID(sourceID), now.Add(3*time.Second)); err != nil {
		t.Fatalf("change dismissed matching Profile: %v", err)
	}
	createAndRunMatchingAnalysis(t, ctx, store, actorID, "matching-analysis-0003", now.Add(4*time.Second))
	refreshed, err := store.GetCase(ctx, matchingCase.ID)
	if err != nil || refreshed.State != CasePending || (refreshed.LeftProfileVersion != 2 && refreshed.RightProfileVersion != 2) {
		t.Fatalf("version-refreshed matching case = %#v, error=%v", refreshed, err)
	}

	insertMatchingDependencies(t, ctx, pool, actorID, survivorID, sourceID, documentTypeID, documentID,
		billTypeID, billID, customFieldID, customValueID, survivorCustomValueID, attachmentFieldID, intentID, attachmentID, entityTypeID, entityID, key, now)
	survivorVersion, sourceVersion := int64(1), int64(2)
	previewInput := MergePreviewInput{
		CaseID: refreshed.ID, SurvivorID: survivorID, SourceID: sourceID,
		SurvivorVersion: survivorVersion, SourceVersion: sourceVersion,
	}
	initialPreview, err := store.PreviewMerge(ctx, previewInput, now.Add(5*time.Second))
	if err != nil || initialPreview.UnresolvedFieldCount == 0 || len(initialPreview.Conflicts) != 0 {
		t.Fatalf("initial PreviewMerge() = %#v, error=%v", initialPreview, err)
	}
	for _, field := range initialPreview.Fields {
		if field.ChoiceRequired {
			previewInput.Choices = append(previewInput.Choices, FieldChoice{FieldKey: field.Key, Source: FieldFromSource})
		}
	}
	preview, err := store.PreviewMerge(ctx, previewInput, now.Add(6*time.Second))
	if err != nil || preview.UnresolvedFieldCount != 0 || preview.PreviewFingerprint == ([32]byte{}) ||
		dependencyCount(preview.Dependencies, DependencyDocumentOwner) != 1 ||
		dependencyCount(preview.Dependencies, DependencyAttachment) != 1 {
		t.Fatalf("resolved PreviewMerge() = %#v, error=%v", preview, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE documents SET version=version+1,updated_at=$2 WHERE id=$1`, matchingUUID(documentID), now.Add(7*time.Second)); err != nil {
		t.Fatalf("change preview dependency topology: %v", err)
	}
	if _, _, err := store.Merge(ctx, actorID, MergeInput{
		MergePreviewInput: previewInput, PreviewFingerprint: preview.PreviewFingerprint,
		IdempotencyKey: "matching-stale-preview", Confirmation: preview.Confirmation,
	}, "merge-stale-preview", now.Add(8*time.Second)); !errors.Is(err, ErrStalePreview) {
		t.Fatalf("Merge(stale dependency preview) error = %v", err)
	}
	var sourceAfterStale, pendingAfterStale int
	if err := pool.QueryRow(ctx, `SELECT
  (SELECT count(*) FROM profiles WHERE id=$1),
  (SELECT count(*) FROM matching_cases WHERE id=$2 AND state='PENDING')`,
		matchingUUID(sourceID), matchingUUID(refreshed.ID)).Scan(&sourceAfterStale, &pendingAfterStale); err != nil {
		t.Fatalf("verify stale preview rollback: %v", err)
	}
	if sourceAfterStale != 1 || pendingAfterStale != 1 {
		t.Fatalf("stale preview mutated merge state: source=%d pending=%d", sourceAfterStale, pendingAfterStale)
	}
	preview, err = store.PreviewMerge(ctx, previewInput, now.Add(9*time.Second))
	if err != nil || preview.UnresolvedFieldCount != 0 || len(preview.Conflicts) != 0 {
		t.Fatalf("refreshed PreviewMerge() = %#v, error=%v", preview, err)
	}
	mergeInput := MergeInput{
		MergePreviewInput: previewInput, PreviewFingerprint: preview.PreviewFingerprint,
		IdempotencyKey: "matching-merge-0001", Confirmation: preview.Confirmation,
	}
	competitor := mergeInput
	competitor.IdempotencyKey = "matching-merge-0002"
	type mergeAttempt struct {
		input   MergeInput
		result  MergeResult
		created bool
		err     error
	}
	start := make(chan struct{})
	attempts := make(chan mergeAttempt, 2)
	for index, input := range []MergeInput{mergeInput, competitor} {
		go func(sequence int, candidate MergeInput) {
			<-start
			result, created, mergeErr := store.Merge(ctx, actorID, candidate, "merge-concurrent", now.Add(time.Duration(10+sequence)*time.Second))
			attempts <- mergeAttempt{input: candidate, result: result, created: created, err: mergeErr}
		}(index, input)
	}
	close(start)
	var winner mergeAttempt
	successes := 0
	for range 2 {
		attempt := <-attempts
		if attempt.err == nil && attempt.created {
			winner, successes = attempt, successes+1
			continue
		}
		if !errors.Is(attempt.err, ErrConflict) && !errors.Is(attempt.err, ErrNotFound) &&
			!errors.Is(attempt.err, ErrInvalidState) && !errors.Is(attempt.err, ErrStalePreview) {
			t.Fatalf("unexpected concurrent merge loser: error=%T %v; attempt=%#v", attempt.err, attempt.err, attempt)
		}
	}
	if successes != 1 || winner.result.SurvivorProfileID != survivorID || winner.result.SourceProfileID != sourceID || winner.result.SurvivorVersion != 2 {
		t.Fatalf("concurrent Merge winner = %#v, successes=%d", winner, successes)
	}
	replayed, created, err := store.Merge(ctx, actorID, winner.input, "merge-replay", now.Add(12*time.Second))
	if err != nil || created || replayed.SurvivorProfileID != survivorID || len(replayed.MovedDependencies) != 8 {
		t.Fatalf("Merge(replay) = %#v, created=%v, error=%v", replayed, created, err)
	}
	reversed := winner.input
	reversed.IdempotencyKey = "matching-merge-reversed"
	reversed.SurvivorID, reversed.SourceID = sourceID, survivorID
	reversed.SurvivorVersion, reversed.SourceVersion = sourceVersion, winner.result.SurvivorVersion
	if _, _, err := store.Merge(ctx, actorID, reversed, "merge-reversed", now.Add(13*time.Second)); !errors.Is(err, ErrNotFound) && !errors.Is(err, ErrInvalidState) && !errors.Is(err, ErrConflict) {
		t.Fatalf("reversed Merge error = %v", err)
	}
	assertMatchingMergeState(t, ctx, pool, refreshed.ID, survivorID, sourceID, documentID, billID,
		customValueID, survivorCustomValueID, intentID, attachmentID, entityID)
}

func TestPostgresMatchingMergeBlocksDependencyCollisionWithoutMutation(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for Matching PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("pgxpool.New() error = %v", err)
	}
	defer pool.Close()
	now := time.Now().UTC().Truncate(time.Second)
	actorID := auth.Identifier(newTestIdentifier())
	firstID, secondID := newTestIdentifier(), newTestIdentifier()
	typeID, firstDocumentID, secondDocumentID := newTestIdentifier(), newTestIdentifier(), newTestIdentifier()
	entityTypeID, firstEntityID, secondEntityID := newTestIdentifier(), newTestIdentifier(), newTestIdentifier()
	key := "matching_conflict_" + firstID.String()[0:8]
	githubID := now.UnixNano() + 97
	if _, err := pool.Exec(ctx, `INSERT INTO app_users(id,github_user_id,github_login,display_name,role,active)
VALUES($1,$2,$3,'Matching Conflict','ADMIN',true)`, matchingAuthUUID(actorID), githubID, key); err != nil {
		t.Fatalf("insert conflict actor: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO profiles(id,full_name,email) VALUES
($1,'Pessoa Duplicada',$3),($2,'Pessoa Duplicada',$3)`, matchingUUID(firstID), matchingUUID(secondID), key+"@example.org"); err != nil {
		t.Fatalf("insert conflict Profiles: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO document_types(id,technical_key,label,active,uniqueness_policy,date_required)
VALUES($1,$2,'Conflict document',true,'PER_PROFILE',false)`, matchingUUID(typeID), key+"_document"); err != nil {
		t.Fatalf("insert conflict document type: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO documents(id,owner_profile_id,document_type_id,identifier_value,uniqueness_policy)
VALUES($1,$3,$5,'SAME-ID','PER_PROFILE'),($2,$4,$5,'SAME-ID','PER_PROFILE')`,
		matchingUUID(firstDocumentID), matchingUUID(secondDocumentID), matchingUUID(firstID), matchingUUID(secondID), matchingUUID(typeID)); err != nil {
		t.Fatalf("insert conflicting documents: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO custom_entity_types(id,technical_key,label,active,profile_cardinality)
VALUES($1,$2,'Conflict entity',true,'ONE_PER_PROFILE')`, matchingUUID(entityTypeID), key+"_entity"); err != nil {
		t.Fatalf("insert conflict entity type: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO custom_entities(id,custom_entity_type_id,owner_profile_id,profile_cardinality)
VALUES($1,$3,$4,'ONE_PER_PROFILE'),($2,$3,$5,'ONE_PER_PROFILE')`,
		matchingUUID(firstEntityID), matchingUUID(secondEntityID), matchingUUID(entityTypeID), matchingUUID(firstID), matchingUUID(secondID)); err != nil {
		t.Fatalf("insert conflicting custom entities: %v", err)
	}
	defer func() {
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, `DELETE FROM matching_audit_events WHERE actor_user_id=$1`, matchingAuthUUID(actorID))
		_, _ = pool.Exec(cleanup, `DELETE FROM matching_merge_receipts WHERE actor_user_id=$1`, matchingAuthUUID(actorID))
		_, _ = pool.Exec(cleanup, `DELETE FROM matching_case_decisions WHERE actor_user_id=$1`, matchingAuthUUID(actorID))
		_, _ = pool.Exec(cleanup, `DELETE FROM matching_analyses WHERE actor_user_id=$1`, matchingAuthUUID(actorID))
		_, _ = pool.Exec(cleanup, `DELETE FROM matching_cases WHERE left_profile_id IN ($1,$2) OR right_profile_id IN ($1,$2)`, matchingUUID(firstID), matchingUUID(secondID))
		_, _ = pool.Exec(cleanup, `DELETE FROM documents WHERE id IN ($1,$2)`, matchingUUID(firstDocumentID), matchingUUID(secondDocumentID))
		_, _ = pool.Exec(cleanup, `DELETE FROM custom_entities WHERE id IN ($1,$2)`, matchingUUID(firstEntityID), matchingUUID(secondEntityID))
		_, _ = pool.Exec(cleanup, `DELETE FROM custom_entity_types WHERE id=$1`, matchingUUID(entityTypeID))
		_, _ = pool.Exec(cleanup, `DELETE FROM document_types WHERE id=$1`, matchingUUID(typeID))
		_, _ = pool.Exec(cleanup, `DELETE FROM profiles WHERE id IN ($1,$2)`, matchingUUID(firstID), matchingUUID(secondID))
		_, _ = pool.Exec(cleanup, `DELETE FROM matching_rate_limits WHERE actor_user_id=$1`, matchingAuthUUID(actorID))
		_, _ = pool.Exec(cleanup, `DELETE FROM app_users WHERE id=$1`, matchingAuthUUID(actorID))
	}()
	store := NewPostgresStore(pool)
	createAndRunMatchingAnalysis(t, ctx, store, actorID, "matching-conflict-analysis", now)
	page, err := store.ListCases(ctx, CaseListOptions{States: []CaseState{CasePending}, Limit: 100})
	if err != nil {
		t.Fatalf("ListCases(conflict) error = %v", err)
	}
	matchingCase := findPair(t, page.Cases, firstID, secondID)
	previewInput := MergePreviewInput{
		CaseID: matchingCase.ID, SurvivorID: firstID, SourceID: secondID,
		SurvivorVersion: 1, SourceVersion: 1,
	}
	preview, err := store.PreviewMerge(ctx, previewInput, now.Add(time.Second))
	if err != nil || len(preview.Conflicts) != 2 ||
		dependencyConflictCount(preview.Conflicts, ConflictDocumentUnique) != 1 ||
		dependencyConflictCount(preview.Conflicts, ConflictCustomEntity) != 1 {
		t.Fatalf("conflicting PreviewMerge() = %#v, error=%v", preview, err)
	}
	_, _, err = store.Merge(ctx, actorID, MergeInput{
		MergePreviewInput: previewInput, PreviewFingerprint: preview.PreviewFingerprint,
		IdempotencyKey: "matching-conflict-merge", Confirmation: preview.Confirmation,
	}, "conflict-merge", now.Add(2*time.Second))
	if !errors.Is(err, ErrDependencyConflict) {
		t.Fatalf("Merge(dependency conflict) error = %v", err)
	}
	var profiles, documents, entities int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM profiles WHERE id IN ($1,$2)`, matchingUUID(firstID), matchingUUID(secondID)).Scan(&profiles); err != nil {
		t.Fatalf("count Profiles after blocked merge: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM documents WHERE id IN ($1,$2)`, matchingUUID(firstDocumentID), matchingUUID(secondDocumentID)).Scan(&documents); err != nil {
		t.Fatalf("count documents after blocked merge: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM custom_entities WHERE id IN ($1,$2)`, matchingUUID(firstEntityID), matchingUUID(secondEntityID)).Scan(&entities); err != nil {
		t.Fatalf("count custom entities after blocked merge: %v", err)
	}
	if profiles != 2 || documents != 2 || entities != 2 {
		t.Fatalf("blocked merge mutated data: profiles=%d documents=%d entities=%d", profiles, documents, entities)
	}
}

func createAndRunMatchingAnalysis(t *testing.T, ctx context.Context, store *PostgresStore, actorID auth.Identifier, key string, now time.Time) Identifier {
	return createAndRunMatchingAnalysisWithMaximum(t, ctx, store, actorID, key, now, MaximumCandidates)
}

func createAndRunMatchingAnalysisWithMaximum(t *testing.T, ctx context.Context, store *PostgresStore, actorID auth.Identifier, key string, now time.Time, maximum int) Identifier {
	t.Helper()
	id := newTestIdentifier()
	created, fresh, err := store.CreateAnalysis(ctx, CreateAnalysisInput{
		ID: id, ActorUserID: actorID, IdempotencyKey: key, ExpiresAt: now.Add(24 * time.Hour),
	}, now.Truncate(AnalysisWindow), 100)
	if err != nil || !fresh || created.State != AnalysisQueued {
		t.Fatalf("CreateAnalysis(%s) = %#v, fresh=%v, error=%v", key, created, fresh, err)
	}
	if _, claimed, err := store.ClaimAnalysis(ctx, id, now); err != nil || !claimed {
		t.Fatalf("ClaimAnalysis(%s) claimed=%v, error=%v", key, claimed, err)
	}
	if _, err := store.GenerateCandidates(ctx, id, maximum, AnalysisTimeout, now); err != nil {
		t.Fatalf("GenerateCandidates(%s) error = %v", key, err)
	}
	return id
}

func matchingAnalysisPairs(t *testing.T, ctx context.Context, pool *pgxpool.Pool, analysisID Identifier) []string {
	t.Helper()
	rows, err := pool.Query(ctx, `SELECT matching_case.left_profile_id::text||'/'||matching_case.right_profile_id::text
FROM matching_analysis_cases analysis_case
JOIN matching_cases matching_case ON matching_case.id=analysis_case.case_id
WHERE analysis_case.analysis_id=$1
ORDER BY matching_case.score DESC, matching_case.left_profile_id, matching_case.right_profile_id`, matchingUUID(analysisID))
	if err != nil {
		t.Fatalf("read matching analysis pairs: %v", err)
	}
	defer rows.Close()
	result := make([]string, 0)
	for rows.Next() {
		var pair string
		if err := rows.Scan(&pair); err != nil {
			t.Fatalf("scan matching analysis pair: %v", err)
		}
		result = append(result, pair)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate matching analysis pairs: %v", err)
	}
	return result
}

func findPair(t *testing.T, cases []Case, first, second Identifier) Case {
	t.Helper()
	for _, value := range cases {
		if samePair(value.LeftProfileID, value.RightProfileID, first, second) {
			return value
		}
	}
	t.Fatalf("matching pair %s/%s not found in %#v", first, second, cases)
	return Case{}
}

func lookupPair(cases []Case, first, second Identifier) (Case, bool) {
	for _, value := range cases {
		if samePair(value.LeftProfileID, value.RightProfileID, first, second) {
			return value, true
		}
	}
	return Case{}, false
}

func hasEvidence(evidence []Evidence, kind EvidenceKind) bool {
	for _, value := range evidence {
		if value.Kind == kind {
			return true
		}
	}
	return false
}

func dependencyCount(values []DependencyCount, kind string) int {
	for _, value := range values {
		if value.Kind == kind {
			return value.Count
		}
	}
	return -1
}

func dependencyConflictCount(values []DependencyConflict, kind string) int {
	for _, value := range values {
		if value.Kind == kind {
			return value.Count
		}
	}
	return -1
}

func insertMatchingDependencies(t *testing.T, ctx context.Context, pool *pgxpool.Pool, actorID auth.Identifier, survivorID, sourceID Identifier,
	documentTypeID, documentID, billTypeID, billID, customFieldID, customValueID, survivorCustomValueID, attachmentFieldID, intentID, attachmentID,
	entityTypeID, entityID Identifier, key string, now time.Time,
) {
	t.Helper()
	statements := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO document_types(id,technical_key,label,active,uniqueness_policy,date_required)
VALUES($1,$2,'Matching document',true,'NONE',false)`, []any{matchingUUID(documentTypeID), key + "_document"}},
		{`INSERT INTO documents(id,owner_profile_id,document_type_id,identifier_value,uniqueness_policy)
VALUES($1,$2,$3,'MATCH-DOC','NONE')`, []any{matchingUUID(documentID), matchingUUID(sourceID), matchingUUID(documentTypeID)}},
		{`INSERT INTO document_current_uses(document_id,holder_profile_id) VALUES($1,$2)`, []any{matchingUUID(documentID), matchingUUID(sourceID)}},
		{`INSERT INTO bill_types(id,technical_key,label,active,supports_current_use)
VALUES($1,$2,'Matching bill',true,true)`, []any{matchingUUID(billTypeID), key + "_bill"}},
		{`INSERT INTO bills(id,owner_profile_id,bill_type_id,reference_value) VALUES($1,$2,$3,'MATCH-BILL')`, []any{matchingUUID(billID), matchingUUID(sourceID), matchingUUID(billTypeID)}},
		{`INSERT INTO bill_current_uses(bill_id,holder_profile_id) VALUES($1,$2)`, []any{matchingUUID(billID), matchingUUID(sourceID)}},
		{`INSERT INTO custom_field_definitions(id,target_kind,technical_key,label,field_kind,required,active)
VALUES($1,'PROFILE',$2,'Matching code','TEXT',false,true)`, []any{matchingUUID(customFieldID), key + "_code"}},
		{`INSERT INTO custom_field_values(id,field_definition_id,profile_id,field_kind,text_value)
VALUES($1,$3,$4,'TEXT','source-code'),($2,$3,$5,'TEXT','survivor-code')`,
			[]any{matchingUUID(customValueID), matchingUUID(survivorCustomValueID), matchingUUID(customFieldID), matchingUUID(sourceID), matchingUUID(survivorID)}},
		{`INSERT INTO custom_field_definitions(id,target_kind,technical_key,label,field_kind,required,active)
VALUES($1,'PROFILE',$2,'Matching file','ATTACHMENT',false,true)`, []any{matchingUUID(attachmentFieldID), key + "_file"}},
		{`INSERT INTO attachment_upload_intents
(id,actor_user_id,owner_kind,custom_target_kind,custom_profile_id,field_definition_id,
 original_filename,declared_mime,expected_size,object_key,expires_at)
VALUES($1,$2,'CUSTOM_FIELD','PROFILE',$3,$4,'pending.txt','text/plain',4,$5,$6)`,
			[]any{matchingUUID(intentID), matchingAuthUUID(actorID), matchingUUID(sourceID), matchingUUID(attachmentFieldID), key + "/pending", now.Add(time.Hour)}},
		{`INSERT INTO attachments
(id,owner_kind,custom_target_kind,custom_profile_id,field_definition_id,original_filename,
 declared_mime,detected_mime,byte_size,sha256,object_key,lifecycle_state)
VALUES($1,'CUSTOM_FIELD','PROFILE',$2,$3,'saved.txt','text/plain','text/plain',4,decode(repeat('01',32),'hex'),$4,'ACTIVE')`,
			[]any{matchingUUID(attachmentID), matchingUUID(sourceID), matchingUUID(attachmentFieldID), key + "/saved"}},
		{`INSERT INTO custom_entity_types(id,technical_key,label,active,profile_cardinality)
VALUES($1,$2,'Matching item',true,'MANY_PER_PROFILE')`, []any{matchingUUID(entityTypeID), key + "_entity"}},
		{`INSERT INTO custom_entities(id,custom_entity_type_id,owner_profile_id,profile_cardinality)
VALUES($1,$2,$3,'MANY_PER_PROFILE')`, []any{matchingUUID(entityID), matchingUUID(entityTypeID), matchingUUID(sourceID)}},
	}
	for _, statement := range statements {
		if _, err := pool.Exec(ctx, statement.query, statement.args...); err != nil {
			t.Fatalf("insert matching dependency: %v", err)
		}
	}
}

func assertMatchingMergeState(t *testing.T, ctx context.Context, pool *pgxpool.Pool, caseID, survivorID, sourceID,
	documentID, billID, customValueID, discardedCustomValueID, intentID, attachmentID, entityID Identifier,
) {
	t.Helper()
	var sourceCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM profiles WHERE id=$1`, matchingUUID(sourceID)).Scan(&sourceCount); err != nil || sourceCount != 0 {
		t.Fatalf("absorbed Profile count = %d, error=%v", sourceCount, err)
	}
	checks := []struct {
		query string
		id    Identifier
	}{
		{`SELECT owner_profile_id=$2 FROM documents WHERE id=$1`, documentID},
		{`SELECT holder_profile_id=$2 FROM document_current_uses WHERE document_id=$1`, documentID},
		{`SELECT owner_profile_id=$2 FROM bills WHERE id=$1`, billID},
		{`SELECT holder_profile_id=$2 FROM bill_current_uses WHERE bill_id=$1`, billID},
		{`SELECT profile_id=$2 FROM custom_field_values WHERE id=$1`, customValueID},
		{`SELECT custom_profile_id=$2 FROM attachment_upload_intents WHERE id=$1`, intentID},
		{`SELECT custom_profile_id=$2 FROM attachments WHERE id=$1`, attachmentID},
		{`SELECT owner_profile_id=$2 FROM custom_entities WHERE id=$1`, entityID},
	}
	for _, check := range checks {
		var moved bool
		if err := pool.QueryRow(ctx, check.query, matchingUUID(check.id), matchingUUID(survivorID)).Scan(&moved); err != nil || !moved {
			t.Fatalf("dependency %s moved=%v, error=%v", check.id, moved, err)
		}
	}
	var selectedCustomValue string
	var discardedCustomValues int
	if err := pool.QueryRow(ctx, `SELECT text_value FROM custom_field_values WHERE id=$1 AND profile_id=$2`, matchingUUID(customValueID), matchingUUID(survivorID)).Scan(&selectedCustomValue); err != nil {
		t.Fatalf("read selected custom Profile value: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM custom_field_values WHERE id=$1`, matchingUUID(discardedCustomValueID)).Scan(&discardedCustomValues); err != nil {
		t.Fatalf("count discarded custom Profile value: %v", err)
	}
	if selectedCustomValue != "source-code" || discardedCustomValues != 0 {
		t.Fatalf("custom Profile choice = %q, discarded=%d", selectedCustomValue, discardedCustomValues)
	}
	var state CaseState
	var databaseSurvivor, databaseSource pgtype.UUID
	var profileName string
	if err := pool.QueryRow(ctx, `SELECT state,merged_survivor_id,merged_source_id FROM matching_cases WHERE id=$1`, matchingUUID(caseID)).Scan(&state, &databaseSurvivor, &databaseSource); err != nil {
		t.Fatalf("read merged matching case: %v", err)
	}
	mergedSurvivor, mergedSource := matchingIdentifier(databaseSurvivor), matchingIdentifier(databaseSource)
	if err := pool.QueryRow(ctx, `SELECT full_name FROM profiles WHERE id=$1`, matchingUUID(survivorID)).Scan(&profileName); err != nil {
		t.Fatalf("read survivor Profile: %v", err)
	}
	if state != CaseMerged || mergedSurvivor != survivorID || mergedSource != sourceID || profileName != "Ana Maria de Souza" {
		t.Fatalf("merged state=%s survivor=%s source=%s name=%q", state, mergedSurvivor, mergedSource, profileName)
	}
	var matchingAudits, profileAudits, decisions, receipts int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM matching_audit_events
WHERE case_id=$1 AND event_type='MERGE_COMPLETED' AND outcome='SUCCESS'`, matchingUUID(caseID)).Scan(&matchingAudits); err != nil {
		t.Fatalf("count matching merge audits: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM profile_audit_events
WHERE profile_id=$1 AND source_profile_id=$2 AND event_type='PROFILE_MERGED' AND outcome='SUCCESS'`,
		matchingUUID(survivorID), matchingUUID(sourceID)).Scan(&profileAudits); err != nil {
		t.Fatalf("count Profile merge audits: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM matching_case_decisions
WHERE case_id=$1 AND action='MERGED'`, matchingUUID(caseID)).Scan(&decisions); err != nil {
		t.Fatalf("count merge decisions: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM matching_merge_receipts
WHERE case_id=$1`, matchingUUID(caseID)).Scan(&receipts); err != nil {
		t.Fatalf("count merge receipts: %v", err)
	}
	if matchingAudits != 1 || profileAudits != 1 || decisions != 1 || receipts != 1 {
		t.Fatalf("durable merge evidence: matching_audits=%d profile_audits=%d decisions=%d receipts=%d",
			matchingAudits, profileAudits, decisions, receipts)
	}
}

func cleanupMatchingCandidateVectors(t *testing.T, pool *pgxpool.Pool, actorID auth.Identifier, profileIDs []Identifier) {
	t.Helper()
	ctx := context.Background()
	_, _ = pool.Exec(ctx, `DELETE FROM matching_audit_events WHERE actor_user_id=$1`, matchingAuthUUID(actorID))
	_, _ = pool.Exec(ctx, `DELETE FROM matching_analyses WHERE actor_user_id=$1`, matchingAuthUUID(actorID))
	_, _ = pool.Exec(ctx, `DELETE FROM matching_cases WHERE left_profile_id=ANY($1::uuid[]) OR right_profile_id=ANY($1::uuid[])`, identifierStrings(profileIDs))
	_, _ = pool.Exec(ctx, `DELETE FROM profiles WHERE id=ANY($1::uuid[])`, identifierStrings(profileIDs))
	_, _ = pool.Exec(ctx, `DELETE FROM matching_rate_limits WHERE actor_user_id=$1`, matchingAuthUUID(actorID))
	_, _ = pool.Exec(ctx, `DELETE FROM app_users WHERE id=$1`, matchingAuthUUID(actorID))
}

func cleanupMatchingIntegration(t *testing.T, pool *pgxpool.Pool, actorID auth.Identifier, profileIDs, definitionIDs []Identifier) {
	t.Helper()
	ctx := context.Background()
	_, _ = pool.Exec(ctx, `DELETE FROM matching_audit_events WHERE actor_user_id=$1`, matchingAuthUUID(actorID))
	_, _ = pool.Exec(ctx, `DELETE FROM matching_merge_receipts WHERE actor_user_id=$1`, matchingAuthUUID(actorID))
	_, _ = pool.Exec(ctx, `DELETE FROM matching_case_decisions WHERE actor_user_id=$1`, matchingAuthUUID(actorID))
	_, _ = pool.Exec(ctx, `DELETE FROM matching_analyses WHERE actor_user_id=$1`, matchingAuthUUID(actorID))
	_, _ = pool.Exec(ctx, `DELETE FROM matching_cases WHERE left_profile_id=ANY($1::uuid[]) OR right_profile_id=ANY($1::uuid[])`, identifierStrings(profileIDs))
	_, _ = pool.Exec(ctx, `DELETE FROM attachment_upload_intents WHERE actor_user_id=$1`, matchingAuthUUID(actorID))
	_, _ = pool.Exec(ctx, `DELETE FROM attachments WHERE object_key LIKE 'matching_%'`)
	_, _ = pool.Exec(ctx, `DELETE FROM custom_field_values WHERE profile_id=ANY($1::uuid[])`, identifierStrings(profileIDs))
	_, _ = pool.Exec(ctx, `DELETE FROM custom_entities WHERE owner_profile_id=ANY($1::uuid[])`, identifierStrings(profileIDs))
	_, _ = pool.Exec(ctx, `DELETE FROM document_current_uses WHERE holder_profile_id=ANY($1::uuid[])`, identifierStrings(profileIDs))
	_, _ = pool.Exec(ctx, `DELETE FROM bill_current_uses WHERE holder_profile_id=ANY($1::uuid[])`, identifierStrings(profileIDs))
	_, _ = pool.Exec(ctx, `DELETE FROM documents WHERE owner_profile_id=ANY($1::uuid[])`, identifierStrings(profileIDs))
	_, _ = pool.Exec(ctx, `DELETE FROM bills WHERE owner_profile_id=ANY($1::uuid[])`, identifierStrings(profileIDs))
	_, _ = pool.Exec(ctx, `DELETE FROM custom_field_definitions WHERE id=ANY($1::uuid[])`, identifierStrings(definitionIDs[2:4]))
	_, _ = pool.Exec(ctx, `DELETE FROM custom_entity_types WHERE id=$1`, matchingUUID(definitionIDs[4]))
	_, _ = pool.Exec(ctx, `DELETE FROM document_types WHERE id=$1`, matchingUUID(definitionIDs[0]))
	_, _ = pool.Exec(ctx, `DELETE FROM bill_types WHERE id=$1`, matchingUUID(definitionIDs[1]))
	_, _ = pool.Exec(ctx, `DELETE FROM profile_audit_events WHERE profile_id=ANY($1::uuid[]) OR source_profile_id=ANY($1::uuid[])`, identifierStrings(profileIDs))
	_, _ = pool.Exec(ctx, `DELETE FROM profiles WHERE id=ANY($1::uuid[])`, identifierStrings(profileIDs))
	_, _ = pool.Exec(ctx, `DELETE FROM matching_rate_limits WHERE actor_user_id=$1`, matchingAuthUUID(actorID))
	_, _ = pool.Exec(ctx, `DELETE FROM app_users WHERE id=$1`, matchingAuthUUID(actorID))
}

func identifierStrings(values []Identifier) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = value.String()
	}
	return result
}

func TestNormalizePostgresMatchingErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{name: "deadline", err: context.DeadlineExceeded, want: ErrTimeout},
		{name: "statement timeout", err: fmt.Errorf("query: %w", &pgconn.PgError{Code: "57014"}), want: ErrTimeout},
		{name: "serialization", err: fmt.Errorf("merge: %w", &pgconn.PgError{Code: "40001"}), want: ErrConflict},
		{name: "opaque serialization wrapper", err: errors.New("driver wrapper (SQLSTATE 40001)"), want: ErrConflict},
		{name: "deadlock", err: fmt.Errorf("merge: %w", &pgconn.PgError{Code: "40P01"}), want: ErrConflict},
		{name: "commit rollback", err: pgx.ErrTxCommitRollback, want: ErrConflict},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := normalizePostgresError(test.err); !errors.Is(err, test.want) {
				t.Fatalf("normalizePostgresError(%v) = %v, want %v", test.err, err, test.want)
			}
		})
	}
}
