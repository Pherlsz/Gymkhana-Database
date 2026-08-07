//go:build integration

package operations

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/attachment"
	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresOperationLifecycleIdempotencyOwnershipExportAndBulkDelete(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for operations PostgreSQL integration tests")
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
	otherActorID, _ := auth.NewIdentifier()
	bulkOne, _ := NewIdentifier()
	bulkTwo, _ := NewIdentifier()
	customFieldID, _ := NewIdentifier()
	key := "operations_" + strings.ReplaceAll(actorID.String(), "-", "")[:18]
	githubID := time.Now().UnixNano()
	if githubID < 0 {
		githubID = -githubID
	}
	if githubID < 2 {
		githubID = 2
	}
	insertOperationActor(t, ctx, pool, actorID, githubID, key, auth.RoleAdmin)
	insertOperationActor(t, ctx, pool, otherActorID, githubID+1, key+"_other", auth.RoleExternal)
	if _, err := pool.Exec(ctx, `INSERT INTO custom_field_definitions
(id, target_kind, technical_key, label, field_kind, required, active)
VALUES($1,'PROFILE','member_code','Código de associado','TEXT',false,true)`, databaseUUID(customFieldID)); err != nil {
		t.Fatalf("insert operation custom field: %v", err)
	}
	defer func() {
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, `DELETE FROM operation_audit_events WHERE actor_user_id IN ($1,$2)`, authDatabaseUUID(actorID), authDatabaseUUID(otherActorID))
		_, _ = pool.Exec(cleanup, `DELETE FROM operation_imports WHERE actor_user_id IN ($1,$2)`, authDatabaseUUID(actorID), authDatabaseUUID(otherActorID))
		_, _ = pool.Exec(cleanup, `DELETE FROM operation_exports WHERE actor_user_id IN ($1,$2)`, authDatabaseUUID(actorID), authDatabaseUUID(otherActorID))
		_, _ = pool.Exec(cleanup, `DELETE FROM operation_rate_limits WHERE actor_user_id IN ($1,$2)`, authDatabaseUUID(actorID), authDatabaseUUID(otherActorID))
		_, _ = pool.Exec(cleanup, `DELETE FROM profiles WHERE full_name LIKE $1 OR id IN ($2,$3)`, key+"%", databaseUUID(bulkOne), databaseUUID(bulkTwo))
		_, _ = pool.Exec(cleanup, `DELETE FROM custom_field_definitions WHERE id=$1`, databaseUUID(customFieldID))
		_, _ = pool.Exec(cleanup, `DELETE FROM app_users WHERE id IN ($1,$2)`, authDatabaseUUID(actorID), authDatabaseUUID(otherActorID))
	}()

	objects := newMemoryOperationObjects()
	jobs := &memoryOperationJobs{}
	fixedNow := time.Date(2026, time.July, 17, 12, 0, 0, 0, time.UTC)
	operationStore := NewPostgresStore(pool)
	service, err := NewService(operationStore, objects, jobs, ServiceOptions{
		Now: fixedNowValue(fixedNow), RateLimit: 100, MaximumActive: 10,
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	actor := auth.Session{User: auth.User{ID: actorID, Role: auth.RoleAdmin, Active: true}}
	otherActor := auth.Session{User: auth.User{ID: otherActorID, Role: auth.RoleExternal, Active: true}}
	catalog, err := service.Catalog(ctx, actor)
	if err != nil || !catalogContainsField(catalog, ModuleProfiles, CustomFieldPrefix+"member_code") {
		t.Fatalf("Catalog() did not expose the logical custom field: %#v, error=%v", catalog, err)
	}

	const concurrentRequests = 8
	replayed := make(chan UploadGrant, concurrentRequests)
	replayErrors := make(chan error, concurrentRequests)
	for index := range concurrentRequests {
		go func(index int) {
			grant, createErr := service.CreateImport(ctx, actor, CreateImportInput{
				Module: ModuleProfiles, OriginalFilename: "replay.xlsx", DeclaredSize: 100,
				IdempotencyKey: "same-import-key",
			}, fmt.Sprintf("integration-replay-%d", index))
			replayed <- grant
			replayErrors <- createErr
		}(index)
	}
	var replayID Identifier
	for range concurrentRequests {
		grant := <-replayed
		if err := <-replayErrors; err != nil {
			t.Fatalf("concurrent CreateImport() error = %v", err)
		}
		if replayID.IsZero() {
			replayID = grant.Import.ID
		} else if grant.Import.ID != replayID {
			t.Fatalf("idempotent import IDs differ: %s != %s", grant.Import.ID, replayID)
		}
	}
	replayedImport, err := service.GetImport(ctx, actor, replayID)
	if err != nil {
		t.Fatalf("GetImport(replay) error = %v", err)
	}
	quotaService, err := NewService(operationStore, objects, jobs, ServiceOptions{
		Now: fixedNowValue(fixedNow), RateLimit: 100, MaximumActive: 1,
	})
	if err != nil {
		t.Fatalf("NewService(quota) error = %v", err)
	}
	quotaInput := CreateImportInput{
		Module: ModuleProfiles, OriginalFilename: "quota.xlsx", DeclaredSize: 100,
		IdempotencyKey: "quota-import-key",
	}
	if _, err := quotaService.CreateImport(ctx, actor, quotaInput, "integration-quota-blocked"); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("active quota CreateImport() error = %v", err)
	}
	if _, err := service.CancelImport(ctx, actor, replayID, replayedImport.Version, "integration-cancel"); err != nil {
		t.Fatalf("CancelImport(replay) error = %v", err)
	}
	quotaGrant, err := quotaService.CreateImport(ctx, actor, quotaInput, "integration-quota-released")
	if err != nil {
		t.Fatalf("released quota CreateImport() error = %v", err)
	}
	if _, err := quotaService.CancelImport(ctx, actor, quotaGrant.Import.ID, quotaGrant.Import.Version, "integration-quota-cancel"); err != nil {
		t.Fatalf("CancelImport(quota) error = %v", err)
	}
	rateService, err := NewService(operationStore, objects, jobs, ServiceOptions{
		Now: fixedNowValue(fixedNow), RateLimit: 1, MaximumActive: 10,
	})
	if err != nil {
		t.Fatalf("NewService(rate limit) error = %v", err)
	}
	rateGrant, err := rateService.CreateImport(ctx, otherActor, CreateImportInput{
		Module: ModuleProfiles, OriginalFilename: "rate.xlsx", DeclaredSize: 100,
		IdempotencyKey: "rate-import-first",
	}, "integration-rate-first")
	if err != nil {
		t.Fatalf("first rate-limited CreateImport() error = %v", err)
	}
	if _, err := rateService.CreateImport(ctx, otherActor, CreateImportInput{
		Module: ModuleProfiles, OriginalFilename: "rate.xlsx", DeclaredSize: 100,
		IdempotencyKey: "rate-import-second",
	}, "integration-rate-second"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("second rate-limited CreateImport() error = %v", err)
	}
	if _, err := rateService.CancelImport(ctx, otherActor, rateGrant.Import.ID, rateGrant.Import.Version, "integration-rate-cancel"); err != nil {
		t.Fatalf("CancelImport(rate limit) error = %v", err)
	}

	data, err := WriteWorkbook("Pessoas", []string{"full_name", "notes", "member_code"}, [][]string{{key + " imported", "=literal-not-formula", "00123"}})
	if err != nil {
		t.Fatalf("WriteWorkbook(import) error = %v", err)
	}
	grant, err := service.CreateImport(ctx, actor, CreateImportInput{
		Module: ModuleProfiles, OriginalFilename: "people.xlsx", DeclaredSize: int64(len(data)),
		IdempotencyKey: "complete-import-key",
	}, "integration-create")
	if err != nil {
		t.Fatalf("CreateImport() error = %v", err)
	}
	objects.set(grant.Import.ObjectKey, data)
	confirmed, err := service.ConfirmImport(ctx, actor, grant.Import.ID, "integration-confirm")
	if err != nil || confirmed.State != ImportParsing {
		t.Fatalf("ConfirmImport() = state %q, error %v", confirmed.State, err)
	}
	if err := service.ParseImport(ctx, grant.Import.ID); err != nil {
		t.Fatalf("ParseImport() error = %v", err)
	}
	if err := service.ParseImport(ctx, grant.Import.ID); err != nil {
		t.Fatalf("duplicate ParseImport() error = %v", err)
	}
	parsed, err := service.GetImport(ctx, actor, grant.Import.ID)
	if err != nil || parsed.State != ImportMapping || len(parsed.Sheets) != 1 {
		t.Fatalf("parsed import = %#v, error = %v", parsed, err)
	}
	selected, err := service.SelectSheet(ctx, actor, parsed.ID, parsed.Version, 0, "integration-sheet")
	if err != nil {
		t.Fatalf("SelectSheet() error = %v", err)
	}
	mapped, err := service.SaveMapping(ctx, actor, selected.ID, selected.Version, []MappingInput{
		{SourceColumn: 0, TargetField: "full_name"},
		{SourceColumn: 1, TargetField: "notes"},
		{SourceColumn: 2, TargetField: CustomFieldPrefix + "member_code"},
	}, "integration-mapping")
	if err != nil {
		t.Fatalf("SaveMapping() error = %v", err)
	}
	preview, err := service.Preview(ctx, actor, mapped.ID, mapped.Version, "integration-preview")
	if err != nil || preview.State != ImportReady || len(preview.Preview) != 1 || preview.Preview[0].ProposedAction != ActionCreate {
		t.Fatalf("Preview() = %#v, error = %v", preview, err)
	}
	queued, err := service.Execute(ctx, actor, preview.ID, preview.Version, "integration-execute")
	if err != nil || queued.State != ImportQueued {
		t.Fatalf("Execute() = state %q, error = %v", queued.State, err)
	}
	running, err := operationStore.BeginImport(ctx, queued.ID, fixedNow)
	if err != nil || running.State != ImportRunning {
		t.Fatalf("BeginImport(crash fixture) = state %q, error=%v", running.State, err)
	}
	if err := service.ExecuteImport(ctx, queued.ID); err != nil {
		t.Fatalf("ExecuteImport(recovery) error = %v", err)
	}
	if err := service.ExecuteImport(ctx, queued.ID); err != nil {
		t.Fatalf("duplicate ExecuteImport() error = %v", err)
	}
	completed, err := service.GetImport(ctx, actor, queued.ID)
	if err != nil || completed.State != ImportCompleted || completed.InsertedCount != 1 {
		t.Fatalf("completed import = %#v, error = %v", completed, err)
	}
	if _, err := service.GetImport(ctx, otherActor, queued.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner GetImport() error = %v", err)
	}
	report, err := service.GetReport(ctx, actor, queued.ID)
	if err != nil || report.State != ImportCompleted || report.Inserted != 1 || len(report.Rows) != 1 || report.Rows[0].Outcome != OutcomeInserted {
		t.Fatalf("GetReport(create) = %#v, error=%v", report, err)
	}
	if _, err := service.GetReport(ctx, otherActor, queued.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner GetReport() error = %v", err)
	}
	var importedCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM profiles WHERE full_name=$1`, key+" imported").Scan(&importedCount); err != nil || importedCount != 1 {
		t.Fatalf("imported canonical row count = %d, error = %v", importedCount, err)
	}
	var importedIDText string
	var importedVersion int64
	var importedNote string
	if err := pool.QueryRow(ctx, `SELECT id::text, version, notes FROM profiles WHERE full_name=$1`, key+" imported").Scan(&importedIDText, &importedVersion, &importedNote); err != nil {
		t.Fatalf("load imported profile: %v", err)
	}
	importedID, err := ParseIdentifier(importedIDText)
	if err != nil {
		t.Fatalf("ParseIdentifier(imported profile) error = %v", err)
	}
	var memberCode string
	if err := pool.QueryRow(ctx, `SELECT text_value FROM custom_field_values WHERE profile_id=$1 AND field_definition_id=$2`, databaseUUID(importedID), databaseUUID(customFieldID)).Scan(&memberCode); err != nil || memberCode != "00123" {
		t.Fatalf("imported custom field = %q, error=%v", memberCode, err)
	}

	pagingRows := make([][]string, MaximumPreviewRows+1)
	for index := range pagingRows {
		pagingRows[index] = []string{importedID.String(), fmt.Sprint(importedVersion), fmt.Sprintf("%s page %03d", key, index)}
	}
	pagingData, err := WriteWorkbook("Pessoas", []string{"record_id", "version", "full_name"}, pagingRows)
	if err != nil {
		t.Fatalf("WriteWorkbook(decision paging) error=%v", err)
	}
	pagingGrant, err := service.CreateImport(ctx, actor, CreateImportInput{
		Module: ModuleProfiles, OriginalFilename: "decision-paging.xlsx", DeclaredSize: int64(len(pagingData)),
		IdempotencyKey: "decision-paging-key",
	}, "integration-decision-paging-create")
	if err != nil {
		t.Fatalf("CreateImport(decision paging) error=%v", err)
	}
	objects.set(pagingGrant.Import.ObjectKey, pagingData)
	if _, err := service.ConfirmImport(ctx, actor, pagingGrant.Import.ID, "integration-decision-paging-confirm"); err != nil {
		t.Fatalf("ConfirmImport(decision paging) error=%v", err)
	}
	if err := service.ParseImport(ctx, pagingGrant.Import.ID); err != nil {
		t.Fatalf("ParseImport(decision paging) error=%v", err)
	}
	pagingImport, err := service.GetImport(ctx, actor, pagingGrant.Import.ID)
	if err != nil {
		t.Fatalf("GetImport(decision paging) error=%v", err)
	}
	pagingImport, err = service.SelectSheet(ctx, actor, pagingImport.ID, pagingImport.Version, 0, "integration-decision-paging-sheet")
	if err != nil {
		t.Fatalf("SelectSheet(decision paging) error=%v", err)
	}
	pagingImport, err = service.SaveMapping(ctx, actor, pagingImport.ID, pagingImport.Version, []MappingInput{
		{SourceColumn: 0, TargetField: "record_id"},
		{SourceColumn: 1, TargetField: "version"},
		{SourceColumn: 2, TargetField: "full_name"},
	}, "integration-decision-paging-mapping")
	if err != nil {
		t.Fatalf("SaveMapping(decision paging) error=%v", err)
	}
	pagingImport, err = service.Preview(ctx, actor, pagingImport.ID, pagingImport.Version, "integration-decision-paging-preview")
	if err != nil || pagingImport.State != ImportDecisionsRequired || pagingImport.UnresolvedCount != MaximumPreviewRows+1 || len(pagingImport.Preview) != MaximumPreviewRows {
		t.Fatalf("Preview(decision paging) = %#v, error=%v", pagingImport, err)
	}
	firstPageDecisions := make([]DecisionInput, 0, len(pagingImport.Preview))
	for _, row := range pagingImport.Preview {
		firstPageDecisions = append(firstPageDecisions, DecisionInput{RowNumber: row.RowNumber, Action: ActionSkip})
	}
	pagingImport, err = service.SaveDecisions(ctx, actor, pagingImport.ID, pagingImport.Version, firstPageDecisions, "integration-decision-paging-first")
	if err != nil || pagingImport.State != ImportDecisionsRequired || pagingImport.UnresolvedCount != 1 || len(pagingImport.Preview) != MaximumPreviewRows || pagingImport.Preview[0].RowNumber != MaximumPreviewRows+2 || pagingImport.Preview[0].Decision != "" {
		t.Fatalf("SaveDecisions(decision paging first) = %#v, error=%v", pagingImport, err)
	}
	pagingImport, err = service.SaveDecisions(ctx, actor, pagingImport.ID, pagingImport.Version, []DecisionInput{{
		RowNumber: pagingImport.Preview[0].RowNumber, Action: ActionSkip,
	}}, "integration-decision-paging-last")
	if err != nil || pagingImport.State != ImportReady || pagingImport.UnresolvedCount != 0 {
		t.Fatalf("SaveDecisions(decision paging last) = %#v, error=%v", pagingImport, err)
	}
	if _, err := service.CancelImport(ctx, actor, pagingImport.ID, pagingImport.Version, "integration-decision-paging-cancel"); err != nil {
		t.Fatalf("CancelImport(decision paging) error=%v", err)
	}

	updateData, err := WriteWorkbook("Pessoas", []string{"record_id", "version", "full_name"}, [][]string{
		{"", "", key + " before conflict"},
		{importedID.String(), fmt.Sprint(importedVersion), key + " updated"},
	})
	if err != nil {
		t.Fatalf("WriteWorkbook(update) error = %v", err)
	}
	updateGrant, err := service.CreateImport(ctx, actor, CreateImportInput{
		Module: ModuleProfiles, OriginalFilename: "people-update.xlsx", DeclaredSize: int64(len(updateData)),
		IdempotencyKey: "update-import-key",
	}, "integration-update-create")
	if err != nil {
		t.Fatalf("CreateImport(update) error = %v", err)
	}
	objects.set(updateGrant.Import.ObjectKey, updateData)
	if _, err := service.ConfirmImport(ctx, actor, updateGrant.Import.ID, "integration-update-confirm"); err != nil {
		t.Fatalf("ConfirmImport(update) error = %v", err)
	}
	if err := service.ParseImport(ctx, updateGrant.Import.ID); err != nil {
		t.Fatalf("ParseImport(update) error = %v", err)
	}
	updateImport, err := service.GetImport(ctx, actor, updateGrant.Import.ID)
	if err != nil {
		t.Fatalf("GetImport(update) error = %v", err)
	}
	updateImport, err = service.SelectSheet(ctx, actor, updateImport.ID, updateImport.Version, 0, "integration-update-sheet")
	if err != nil {
		t.Fatalf("SelectSheet(update) error = %v", err)
	}
	updateImport, err = service.SaveMapping(ctx, actor, updateImport.ID, updateImport.Version, []MappingInput{
		{SourceColumn: 0, TargetField: "record_id"},
		{SourceColumn: 1, TargetField: "version"},
		{SourceColumn: 2, TargetField: "full_name"},
	}, "integration-update-mapping")
	if err != nil {
		t.Fatalf("SaveMapping(update) error = %v", err)
	}
	updateImport, err = service.Preview(ctx, actor, updateImport.ID, updateImport.Version, "integration-update-preview")
	if err != nil || updateImport.State != ImportDecisionsRequired || len(updateImport.Preview) != 2 {
		t.Fatalf("Preview(update) = %#v, error=%v", updateImport, err)
	}
	updateDecision := requiredDecisionRow(t, updateImport.Preview)
	updateImport, err = service.SaveDecisions(ctx, actor, updateImport.ID, updateImport.Version, []DecisionInput{{
		RowNumber: updateDecision.RowNumber, Action: ActionUpdate,
		TargetID: updateDecision.TargetID, Version: updateDecision.TargetVersion,
	}}, "integration-update-decision")
	if err != nil || updateImport.State != ImportReady {
		t.Fatalf("SaveDecisions(update) = %#v, error=%v", updateImport, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE profiles SET version=version+1, updated_at=now() WHERE id=$1`, databaseUUID(importedID)); err != nil {
		t.Fatalf("advance canonical version for conflict: %v", err)
	}
	updateImport, err = service.Execute(ctx, actor, updateImport.ID, updateImport.Version, "integration-update-execute")
	if err != nil {
		t.Fatalf("Execute(update) error = %v", err)
	}
	if err := service.ExecuteImport(ctx, updateImport.ID); !errors.Is(err, ErrStalePreview) {
		t.Fatalf("ExecuteImport(stale update) error = %v", err)
	}
	updateImport, err = service.GetImport(ctx, actor, updateImport.ID)
	if err != nil || updateImport.State != ImportPreviewReady || updateImport.InsertedCount != 1 || updateImport.ConflictedCount != 1 {
		t.Fatalf("stale update was not reopened = %#v, error=%v", updateImport, err)
	}
	conflictReport, err := service.GetReport(ctx, actor, updateImport.ID)
	if err != nil || conflictReport.Inserted != 1 || conflictReport.Conflicted != 1 || len(conflictReport.Rows) != 2 || conflictReport.Rows[0].Outcome != OutcomeInserted || conflictReport.Rows[1].Outcome != OutcomeConflicted {
		t.Fatalf("GetReport(conflict) = %#v, error=%v", conflictReport, err)
	}
	if _, err := service.SaveMapping(ctx, actor, updateImport.ID, updateImport.Version, []MappingInput{
		{SourceColumn: 0, TargetField: "record_id"},
		{SourceColumn: 1, TargetField: "version"},
		{SourceColumn: 2, TargetField: "full_name"},
	}, "integration-update-remap-after-commit"); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("SaveMapping(after committed row) error=%v", err)
	}
	if _, err := service.SelectSheet(ctx, actor, updateImport.ID, updateImport.Version, 0, "integration-update-reselect-after-commit"); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("SelectSheet(after committed row) error=%v", err)
	}
	updateImport, err = service.Preview(ctx, actor, updateImport.ID, updateImport.Version, "integration-update-repreview")
	if err != nil || updateImport.State != ImportDecisionsRequired || updateImport.InsertedCount != 1 || updateImport.ConflictedCount != 0 || len(updateImport.Preview) != 2 {
		t.Fatalf("Preview(reopened update) = %#v, error=%v", updateImport, err)
	}
	updateDecision = requiredDecisionRow(t, updateImport.Preview)
	if updateDecision.TargetVersion != importedVersion+1 {
		t.Fatalf("repreview target version=%d, want %d", updateDecision.TargetVersion, importedVersion+1)
	}
	updateImport, err = service.SaveDecisions(ctx, actor, updateImport.ID, updateImport.Version, []DecisionInput{{
		RowNumber: updateDecision.RowNumber, Action: ActionUpdate,
		TargetID: updateDecision.TargetID, Version: updateDecision.TargetVersion,
	}}, "integration-update-redecision")
	if err != nil || updateImport.State != ImportReady {
		t.Fatalf("SaveDecisions(reopened update) = %#v, error=%v", updateImport, err)
	}
	updateImport, err = service.Execute(ctx, actor, updateImport.ID, updateImport.Version, "integration-update-reexecute")
	if err != nil {
		t.Fatalf("Execute(reopened update) error = %v", err)
	}
	if err := service.ExecuteImport(ctx, updateImport.ID); err != nil {
		t.Fatalf("ExecuteImport(reopened update) error = %v", err)
	}
	var updatedName, preservedNote, preservedCode string
	var linkedTargetVersion int64
	if err := pool.QueryRow(ctx, `SELECT profile.full_name, profile.notes, value.text_value, profile.version
  FROM profiles profile
  JOIN custom_field_values value ON value.profile_id=profile.id AND value.field_definition_id=$2
 WHERE profile.id=$1`, databaseUUID(importedID), databaseUUID(customFieldID)).Scan(&updatedName, &preservedNote, &preservedCode, &linkedTargetVersion); err != nil {
		t.Fatalf("load updated profile: %v", err)
	}
	if updatedName != key+" updated" || preservedNote != importedNote || preservedCode != "00123" {
		t.Fatalf("partial update lost data: name=%q note=%q code=%q", updatedName, preservedNote, preservedCode)
	}
	updateReport, err := service.GetReport(ctx, actor, updateImport.ID)
	if err != nil || updateReport.State != ImportCompleted || updateReport.Inserted != 1 || updateReport.Updated != 1 || updateReport.Decisions != 1 || len(updateReport.Rows) != 2 || updateReport.Rows[0].Outcome != OutcomeInserted || updateReport.Rows[1].Outcome != OutcomeUpdated {
		t.Fatalf("GetReport(update) = %#v, error=%v", updateReport, err)
	}
	var beforeConflictCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM profiles WHERE full_name=$1`, key+" before conflict").Scan(&beforeConflictCount); err != nil || beforeConflictCount != 1 {
		t.Fatalf("committed row was repeated after repreview: count=%d, error=%v", beforeConflictCount, err)
	}

	mixedData, err := WriteWorkbook("Pessoas", []string{"record_id", "version", "full_name"}, [][]string{
		{importedID.String(), fmt.Sprint(linkedTargetVersion), key + " must not replace"},
		{"", "", key + " updated"},
		{"", "", ""},
	})
	if err != nil {
		t.Fatalf("WriteWorkbook(mixed) error = %v", err)
	}
	mixedGrant, err := service.CreateImport(ctx, actor, CreateImportInput{
		Module: ModuleProfiles, OriginalFilename: "people-mixed.xlsx", DeclaredSize: int64(len(mixedData)),
		IdempotencyKey: "mixed-link-import-key",
	}, "integration-mixed-create")
	if err != nil {
		t.Fatalf("CreateImport(mixed) error = %v", err)
	}
	objects.set(mixedGrant.Import.ObjectKey, mixedData)
	if _, err := service.ConfirmImport(ctx, actor, mixedGrant.Import.ID, "integration-mixed-confirm"); err != nil {
		t.Fatalf("ConfirmImport(mixed) error = %v", err)
	}
	if err := service.ParseImport(ctx, mixedGrant.Import.ID); err != nil {
		t.Fatalf("ParseImport(mixed) error = %v", err)
	}
	mixedImport, err := service.GetImport(ctx, actor, mixedGrant.Import.ID)
	if err != nil {
		t.Fatalf("GetImport(mixed) error = %v", err)
	}
	mixedImport, err = service.SelectSheet(ctx, actor, mixedImport.ID, mixedImport.Version, 0, "integration-mixed-sheet")
	if err != nil {
		t.Fatalf("SelectSheet(mixed) error = %v", err)
	}
	mixedImport, err = service.SaveMapping(ctx, actor, mixedImport.ID, mixedImport.Version, []MappingInput{
		{SourceColumn: 0, TargetField: "record_id"},
		{SourceColumn: 1, TargetField: "version"},
		{SourceColumn: 2, TargetField: "full_name"},
	}, "integration-mixed-mapping")
	if err != nil {
		t.Fatalf("SaveMapping(mixed) error = %v", err)
	}
	mixedImport, err = service.Preview(ctx, actor, mixedImport.ID, mixedImport.Version, "integration-mixed-preview")
	if err != nil || mixedImport.State != ImportDecisionsRequired || len(mixedImport.Preview) != 3 || mixedImport.Preview[0].ProposedAction != ActionUpdate || mixedImport.Preview[1].ProposedAction != ActionCreate || mixedImport.Preview[2].ProposedAction != ActionSkip {
		t.Fatalf("Preview(mixed) = %#v, error=%v", mixedImport, err)
	}
	mixedImport, err = service.SaveDecisions(ctx, actor, mixedImport.ID, mixedImport.Version, []DecisionInput{{
		RowNumber: mixedImport.Preview[0].RowNumber, Action: ActionLink,
		TargetID: mixedImport.Preview[0].TargetID, Version: mixedImport.Preview[0].TargetVersion,
	}}, "integration-mixed-link")
	if err != nil || mixedImport.State != ImportReady {
		t.Fatalf("SaveDecisions(mixed) = %#v, error=%v", mixedImport, err)
	}
	mixedImport, err = service.Execute(ctx, actor, mixedImport.ID, mixedImport.Version, "integration-mixed-execute")
	if err != nil {
		t.Fatalf("Execute(mixed) error = %v", err)
	}
	if err := service.ExecuteImport(ctx, mixedImport.ID); err != nil {
		t.Fatalf("ExecuteImport(mixed) error = %v", err)
	}
	mixedReport, err := service.GetReport(ctx, actor, mixedImport.ID)
	if err != nil || mixedReport.Inserted != 1 || mixedReport.Linked != 1 || mixedReport.Skipped != 1 || mixedReport.Decisions != 1 || len(mixedReport.Rows) != 3 {
		t.Fatalf("GetReport(mixed) = %#v, error=%v", mixedReport, err)
	}
	var unchangedName string
	var duplicateNameCount int
	if err := pool.QueryRow(ctx, `SELECT full_name FROM profiles WHERE id=$1`, databaseUUID(importedID)).Scan(&unchangedName); err != nil {
		t.Fatalf("load linked profile: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM profiles WHERE full_name=$1`, key+" updated").Scan(&duplicateNameCount); err != nil {
		t.Fatalf("count non-merged profiles: %v", err)
	}
	if unchangedName != key+" updated" || duplicateNameCount != 2 {
		t.Fatalf("link or duplicate handling mutated/merged profiles: name=%q count=%d", unchangedName, duplicateNameCount)
	}

	cancelledImport := queueProfileCreateImport(t, ctx, service, objects, actor,
		"cancel-between-rows-key", [][]string{{key + " cancellation first"}, {key + " cancellation second"}})
	cancelledRunning, err := operationStore.BeginImport(ctx, cancelledImport.ID, fixedNow)
	if err != nil || cancelledRunning.State != ImportRunning {
		t.Fatalf("BeginImport(cancellation) = %#v, error=%v", cancelledRunning, err)
	}
	pendingRows, err := operationStore.ListPendingRows(ctx, cancelledRunning.ID, MaximumBatchSize)
	if err != nil || len(pendingRows) != 2 {
		t.Fatalf("ListPendingRows(cancellation) rows=%d, error=%v", len(pendingRows), err)
	}
	firstMutation, _, err := service.mutationForRow(ctx, ModuleProfiles, pendingRows[0].Values, nil)
	if err != nil {
		t.Fatalf("mutationForRow(cancellation first) error=%v", err)
	}
	if _, err := operationStore.ApplyRow(ctx, cancelledRunning, pendingRows[0], firstMutation,
		deterministicRowIdentifier(cancelledRunning.ID, pendingRows[0].Row), "integration-cancel-first", fixedNow); err != nil {
		t.Fatalf("ApplyRow(cancellation first) error=%v", err)
	}
	cancelledImport, err = service.CancelImport(ctx, actor, cancelledRunning.ID, cancelledRunning.Version, "integration-cancel-running")
	if err != nil || cancelledImport.State != ImportCancelled {
		t.Fatalf("CancelImport(running) = %#v, error=%v", cancelledImport, err)
	}
	secondMutation, _, err := service.mutationForRow(ctx, ModuleProfiles, pendingRows[1].Values, nil)
	if err != nil {
		t.Fatalf("mutationForRow(cancellation second) error=%v", err)
	}
	if _, err := operationStore.ApplyRow(ctx, cancelledRunning, pendingRows[1], secondMutation,
		deterministicRowIdentifier(cancelledRunning.ID, pendingRows[1].Row), "integration-cancel-second", fixedNow); !errors.Is(err, ErrCancelled) {
		t.Fatalf("ApplyRow(after cancellation) error=%v", err)
	}
	var firstCancellationCount, secondCancellationCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM profiles WHERE full_name=$1`, key+" cancellation first").Scan(&firstCancellationCount); err != nil {
		t.Fatalf("count first cancellation row: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM profiles WHERE full_name=$1`, key+" cancellation second").Scan(&secondCancellationCount); err != nil {
		t.Fatalf("count second cancellation row: %v", err)
	}
	if firstCancellationCount != 1 || secondCancellationCount != 0 || cancelledImport.InsertedCount != 1 {
		t.Fatalf("cancellation did not stop between rows: first=%d second=%d import=%#v", firstCancellationCount, secondCancellationCount, cancelledImport)
	}

	exported, err := service.CreateExport(ctx, actor, ModuleProfiles, "full-profile-export", "integration-export")
	if err != nil {
		t.Fatalf("CreateExport() error = %v", err)
	}
	replayedExport, err := service.CreateExport(ctx, actor, ModuleProfiles, "full-profile-export", "integration-export-replay")
	if err != nil || replayedExport.ID != exported.ID {
		t.Fatalf("idempotent CreateExport() = %s, error = %v", replayedExport.ID, err)
	}
	if err := service.GenerateExport(ctx, exported.ID); err != nil {
		t.Fatalf("GenerateExport() error = %v", err)
	}
	if err := service.GenerateExport(ctx, exported.ID); err != nil {
		t.Fatalf("duplicate GenerateExport() error = %v", err)
	}
	finishedExport, err := service.GetExport(ctx, actor, exported.ID)
	if err != nil || finishedExport.State != ExportCompleted || finishedExport.RowCount < 1 {
		t.Fatalf("completed export = %#v, error = %v", finishedExport, err)
	}
	exportData := objects.get(t, finishedExport.ObjectKey)
	workbook, _, err := ReadWorkbook(bytes.NewReader(exportData), int64(len(exportData)))
	if err != nil {
		t.Fatalf("ReadWorkbook(export) error = %v", err)
	}
	if !workbookContainsLiteral(workbook, "=literal-not-formula") || !workbookContainsLiteral(workbook, "00123") || workbookContainsFormula(workbook) {
		t.Fatal("export lost the literal formula marker or emitted an executable formula")
	}
	if objects.putCount(finishedExport.ObjectKey) != 1 {
		t.Fatalf("duplicate export wrote object %d times", objects.putCount(finishedExport.ObjectKey))
	}

	if _, err := pool.Exec(ctx, `INSERT INTO profiles(id, full_name) VALUES($1,$2),($3,$4)`,
		databaseUUID(bulkOne), key+" bulk one", databaseUUID(bulkTwo), key+" bulk two"); err != nil {
		t.Fatalf("insert bulk profiles: %v", err)
	}
	items := []BulkItem{{ID: bulkOne, Version: 1}, {ID: bulkTwo, Version: 2}}
	if _, err := pool.Exec(ctx, `UPDATE app_users SET role='EXTERNAL' WHERE id=$1`, authDatabaseUUID(actorID)); err != nil {
		t.Fatalf("revoke bulk delete actor: %v", err)
	}
	if _, err := service.BulkDelete(ctx, actor, ModuleProfiles, []BulkItem{{ID: bulkOne, Version: 1}}, BulkDeleteConfirmation, "integration-bulk-revoked"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("revoked BulkDelete() error = %v", err)
	}
	if countOperationProfiles(t, ctx, pool, bulkOne, bulkTwo) != 2 {
		t.Fatal("revoked bulk deletion changed canonical records")
	}
	if _, err := pool.Exec(ctx, `UPDATE app_users SET role='ADMIN' WHERE id=$1`, authDatabaseUUID(actorID)); err != nil {
		t.Fatalf("restore bulk delete actor: %v", err)
	}
	if _, err := service.BulkDelete(ctx, actor, ModuleProfiles, items, BulkDeleteConfirmation, "integration-bulk-conflict"); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale BulkDelete() error = %v", err)
	}
	if countOperationProfiles(t, ctx, pool, bulkOne, bulkTwo) != 2 {
		t.Fatal("conflicted bulk deletion was not rolled back")
	}
	items[1].Version = 1
	if _, err := service.BulkDelete(ctx, actor, ModuleProfiles, items, "confirmar", "integration-bulk-invalid"); !errors.Is(err, ErrInvalidConfirmation) {
		t.Fatalf("invalid confirmation BulkDelete() error = %v", err)
	}
	deleted, err := service.BulkDelete(ctx, actor, ModuleProfiles, items, BulkDeleteConfirmation, "integration-bulk-success")
	if err != nil || deleted.Deleted != 2 || countOperationProfiles(t, ctx, pool, bulkOne, bulkTwo) != 0 {
		t.Fatalf("BulkDelete() = %#v, remaining=%d, error=%v", deleted, countOperationProfiles(t, ctx, pool, bulkOne, bulkTwo), err)
	}

	if _, err := pool.Exec(ctx, `UPDATE operation_imports SET expires_at=$2 WHERE id=$1`, databaseUUID(completed.ID), fixedNow.Add(-time.Minute)); err != nil {
		t.Fatalf("expire import fixture: %v", err)
	}
	cleaned, err := service.Cleanup(ctx)
	if err != nil || cleaned != 1 {
		t.Fatalf("Cleanup() = %d, %v", cleaned, err)
	}
	if objects.exists(completed.ObjectKey) {
		t.Fatal("cleanup retained the expired import object")
	}
	cleaned, err = service.Cleanup(ctx)
	if err != nil || cleaned != 0 {
		t.Fatalf("idempotent Cleanup() = %d, %v", cleaned, err)
	}
	expired, err := service.GetImport(ctx, actor, completed.ID)
	if err != nil || expired.State != ImportExpired {
		t.Fatalf("expired import state = %q, error = %v", expired.State, err)
	}
}

func queueProfileCreateImport(t *testing.T, ctx context.Context, service *Service, objects *memoryOperationObjects, actor auth.Session, idempotencyKey string, rows [][]string) Import {
	t.Helper()
	data, err := WriteWorkbook("Pessoas", []string{"full_name"}, rows)
	if err != nil {
		t.Fatalf("WriteWorkbook(%s) error=%v", idempotencyKey, err)
	}
	grant, err := service.CreateImport(ctx, actor, CreateImportInput{
		Module: ModuleProfiles, OriginalFilename: idempotencyKey + ".xlsx", DeclaredSize: int64(len(data)),
		IdempotencyKey: idempotencyKey,
	}, "integration-"+idempotencyKey+"-create")
	if err != nil {
		t.Fatalf("CreateImport(%s) error=%v", idempotencyKey, err)
	}
	objects.set(grant.Import.ObjectKey, data)
	if _, err := service.ConfirmImport(ctx, actor, grant.Import.ID, "integration-"+idempotencyKey+"-confirm"); err != nil {
		t.Fatalf("ConfirmImport(%s) error=%v", idempotencyKey, err)
	}
	if err := service.ParseImport(ctx, grant.Import.ID); err != nil {
		t.Fatalf("ParseImport(%s) error=%v", idempotencyKey, err)
	}
	value, err := service.GetImport(ctx, actor, grant.Import.ID)
	if err != nil {
		t.Fatalf("GetImport(%s) error=%v", idempotencyKey, err)
	}
	value, err = service.SelectSheet(ctx, actor, value.ID, value.Version, 0, "integration-"+idempotencyKey+"-sheet")
	if err != nil {
		t.Fatalf("SelectSheet(%s) error=%v", idempotencyKey, err)
	}
	value, err = service.SaveMapping(ctx, actor, value.ID, value.Version, []MappingInput{{SourceColumn: 0, TargetField: "full_name"}}, "integration-"+idempotencyKey+"-mapping")
	if err != nil {
		t.Fatalf("SaveMapping(%s) error=%v", idempotencyKey, err)
	}
	value, err = service.Preview(ctx, actor, value.ID, value.Version, "integration-"+idempotencyKey+"-preview")
	if err != nil || value.State != ImportReady {
		t.Fatalf("Preview(%s) = %#v, error=%v", idempotencyKey, value, err)
	}
	value, err = service.Execute(ctx, actor, value.ID, value.Version, "integration-"+idempotencyKey+"-execute")
	if err != nil || value.State != ImportQueued {
		t.Fatalf("Execute(%s) = %#v, error=%v", idempotencyKey, value, err)
	}
	return value
}

func insertOperationActor(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id auth.Identifier, subjectID int64, login string, role auth.Role) {
	t.Helper()
	if _, err := pool.Exec(ctx, `INSERT INTO app_users
(id, subject, email, display_name, role, active)
VALUES($1,$2::text,lower($3) || '@example.test',$4,$5,true)`, authDatabaseUUID(id), subjectID, login, "Operations test actor", role); err != nil {
		t.Fatalf("insert operation actor: %v", err)
	}
}

func fixedNowValue(value time.Time) func() time.Time { return func() time.Time { return value } }

func countOperationProfiles(t *testing.T, ctx context.Context, pool *pgxpool.Pool, first, second Identifier) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM profiles WHERE id=$1 OR id=$2`, databaseUUID(first), databaseUUID(second)).Scan(&count); err != nil {
		t.Fatalf("count operation profiles: %v", err)
	}
	return count
}

func catalogContainsField(catalog []ModuleCatalog, module Module, fieldID string) bool {
	for _, candidate := range catalog {
		if candidate.ID != module {
			continue
		}
		for _, field := range candidate.Fields {
			if field.ID == fieldID {
				return true
			}
		}
	}
	return false
}

func requiredDecisionRow(t *testing.T, rows []Row) Row {
	t.Helper()
	for _, row := range rows {
		if row.DecisionRequired {
			return row
		}
	}
	t.Fatal("preview has no decision-required row")
	return Row{}
}

func workbookContainsLiteral(workbook Workbook, expected string) bool {
	for _, sheet := range workbook.Sheets {
		for _, row := range sheet.Rows {
			for _, cell := range row.Cells {
				if cell.Value == expected {
					return true
				}
			}
		}
	}
	return false
}

func workbookContainsFormula(workbook Workbook) bool {
	for _, sheet := range workbook.Sheets {
		for _, row := range sheet.Rows {
			for _, cell := range row.Cells {
				if cell.FormulaPresent {
					return true
				}
			}
		}
	}
	return false
}

type memoryOperationObjects struct {
	mu      sync.Mutex
	objects map[string][]byte
	puts    map[string]int
}

func newMemoryOperationObjects() *memoryOperationObjects {
	return &memoryOperationObjects{objects: make(map[string][]byte), puts: make(map[string]int)}
}

func (objects *memoryOperationObjects) PresignOperationUpload(_ context.Context, key string, size int64, ttl time.Duration) (attachment.SignedRequest, error) {
	return attachment.SignedRequest{URL: "https://storage.invalid/upload", Method: "PUT", Headers: map[string]string{"content-length": fmt.Sprint(size)}, ExpiresAt: time.Now().Add(ttl)}, nil
}

func (objects *memoryOperationObjects) PresignDownload(_ context.Context, key, filename, mime string, ttl time.Duration) (attachment.SignedRequest, error) {
	return attachment.SignedRequest{URL: "https://storage.invalid/download", Method: "GET", ExpiresAt: time.Now().Add(ttl)}, nil
}

func (objects *memoryOperationObjects) Open(_ context.Context, key string) (io.ReadCloser, error) {
	objects.mu.Lock()
	defer objects.mu.Unlock()
	value, ok := objects.objects[key]
	if !ok {
		return nil, errors.New("object not found")
	}
	return io.NopCloser(bytes.NewReader(append([]byte(nil), value...))), nil
}

func (objects *memoryOperationObjects) Put(_ context.Context, key string, reader io.Reader, size int64, _ string) error {
	value, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	if int64(len(value)) != size {
		return errors.New("object size mismatch")
	}
	objects.mu.Lock()
	defer objects.mu.Unlock()
	objects.objects[key] = append([]byte(nil), value...)
	objects.puts[key]++
	return nil
}

func (objects *memoryOperationObjects) Delete(_ context.Context, key string) error {
	objects.mu.Lock()
	defer objects.mu.Unlock()
	delete(objects.objects, key)
	return nil
}

func (objects *memoryOperationObjects) set(key string, value []byte) {
	objects.mu.Lock()
	defer objects.mu.Unlock()
	objects.objects[key] = append([]byte(nil), value...)
}

func (objects *memoryOperationObjects) get(t *testing.T, key string) []byte {
	t.Helper()
	objects.mu.Lock()
	defer objects.mu.Unlock()
	value, ok := objects.objects[key]
	if !ok {
		t.Fatalf("object %q not found", key)
	}
	return append([]byte(nil), value...)
}

func (objects *memoryOperationObjects) exists(key string) bool {
	objects.mu.Lock()
	defer objects.mu.Unlock()
	_, ok := objects.objects[key]
	return ok
}

func (objects *memoryOperationObjects) putCount(key string) int {
	objects.mu.Lock()
	defer objects.mu.Unlock()
	return objects.puts[key]
}

type memoryOperationJobs struct {
	mu        sync.Mutex
	next      int64
	cancelled []int64
}

func (jobs *memoryOperationJobs) enqueue() (int64, error) {
	jobs.mu.Lock()
	defer jobs.mu.Unlock()
	jobs.next++
	return jobs.next, nil
}

func (jobs *memoryOperationJobs) EnqueueParse(context.Context, Identifier) (int64, error) {
	return jobs.enqueue()
}

func (jobs *memoryOperationJobs) EnqueueExecute(context.Context, Identifier) (int64, error) {
	return jobs.enqueue()
}

func (jobs *memoryOperationJobs) EnqueueExport(context.Context, Identifier) (int64, error) {
	return jobs.enqueue()
}

func (jobs *memoryOperationJobs) EnqueueCleanup(context.Context, time.Time) (int64, error) {
	return jobs.enqueue()
}

func (jobs *memoryOperationJobs) Cancel(_ context.Context, id int64) error {
	jobs.mu.Lock()
	defer jobs.mu.Unlock()
	jobs.cancelled = append(jobs.cancelled, id)
	return nil
}
