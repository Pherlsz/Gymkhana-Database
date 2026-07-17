package matching

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type canonicalMergeField struct {
	key   string
	label string
	kind  string
	value func(ProfileSnapshot) *string
}

var canonicalMergeFields = []canonicalMergeField{
	{key: "full_name", label: "Nome completo", kind: "TEXT", value: func(value ProfileSnapshot) *string { return requiredString(value.FullName) }},
	{key: "social_name", label: "Nome social", kind: "TEXT", value: func(value ProfileSnapshot) *string { return optionalString(value.SocialName) }},
	{key: "cpf", label: "CPF", kind: "IDENTIFIER", value: func(value ProfileSnapshot) *string { return optionalString(value.CPF) }},
	{key: "email", label: "E-mail", kind: "EMAIL", value: func(value ProfileSnapshot) *string { return optionalString(value.Email) }},
	{key: "mobile_phone", label: "Celular", kind: "PHONE", value: func(value ProfileSnapshot) *string { return optionalString(value.MobilePhone) }},
	{key: "landline_phone", label: "Telefone", kind: "PHONE", value: func(value ProfileSnapshot) *string { return optionalString(value.LandlinePhone) }},
	{key: "address_street", label: "Logradouro", kind: "TEXT", value: func(value ProfileSnapshot) *string { return optionalString(value.AddressStreet) }},
	{key: "address_number", label: "Número", kind: "TEXT", value: func(value ProfileSnapshot) *string { return optionalString(value.AddressNumber) }},
	{key: "address_complement", label: "Complemento", kind: "TEXT", value: func(value ProfileSnapshot) *string { return optionalString(value.AddressComplement) }},
	{key: "address_neighborhood", label: "Bairro", kind: "TEXT", value: func(value ProfileSnapshot) *string { return optionalString(value.AddressNeighborhood) }},
	{key: "address_city", label: "Cidade", kind: "TEXT", value: func(value ProfileSnapshot) *string { return optionalString(value.AddressCity) }},
	{key: "address_state", label: "UF", kind: "TEXT", value: func(value ProfileSnapshot) *string { return optionalString(value.AddressState) }},
	{key: "address_postal_code", label: "CEP", kind: "IDENTIFIER", value: func(value ProfileSnapshot) *string { return optionalString(value.AddressPostalCode) }},
	{key: "notes", label: "Observações", kind: "LONG_TEXT", value: func(value ProfileSnapshot) *string { return optionalString(value.Notes) }},
}

type customMergeField struct {
	definitionID    Identifier
	field           MergeField
	survivorValueID *Identifier
	sourceValueID   *Identifier
}

type mergeContext struct {
	preview          MergePreview
	customFields     []customMergeField
	dependencyDigest []string
}

func (store *PostgresStore) PreviewMerge(ctx context.Context, input MergePreviewInput, now time.Time) (MergePreview, error) {
	if store == nil || store.pool == nil || now.IsZero() {
		return MergePreview{}, ErrInvalidInput
	}
	if err := validatePreviewInput(input); err != nil {
		return MergePreview{}, err
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return MergePreview{}, fmt.Errorf("begin merge preview: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('statement_timeout','15000',true)`); err != nil {
		return MergePreview{}, fmt.Errorf("configure merge preview timeout: %w", err)
	}
	value, err := buildMergeContext(ctx, tx, input, now)
	if err != nil {
		return MergePreview{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return MergePreview{}, normalizePostgresError(err)
	}
	return value.preview, nil
}

func (store *PostgresStore) Merge(ctx context.Context, actorID auth.Identifier, input MergeInput, requestID string, now time.Time) (MergeResult, bool, error) {
	if store == nil || store.pool == nil || actorID == (auth.Identifier{}) || now.IsZero() ||
		!strings.HasPrefix(input.Confirmation, MergeConfirmation+" ") || input.PreviewFingerprint == ([sha256.Size]byte{}) ||
		!validIdempotencyKey(input.IdempotencyKey) || len(requestID) < 1 || len(requestID) > 128 {
		return MergeResult{}, false, ErrInvalidInput
	}
	if err := validatePreviewInput(input.MergePreviewInput); err != nil {
		return MergeResult{}, false, err
	}
	requestFingerprint, err := mergeInputFingerprint(input)
	if err != nil {
		return MergeResult{}, false, err
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return MergeResult{}, false, fmt.Errorf("begin profile merge: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('statement_timeout','15000',true)`); err != nil {
		return MergeResult{}, false, fmt.Errorf("configure profile merge timeout: %w", err)
	}
	if replay, found, err := loadMergeReceipt(ctx, tx, actorID, input.IdempotencyKey, requestFingerprint); err != nil {
		return MergeResult{}, false, err
	} else if found {
		if err := tx.Commit(ctx); err != nil {
			return MergeResult{}, false, normalizePostgresError(err)
		}
		return replay, false, nil
	}
	if err := lockProfilesInOrder(ctx, tx, input.SurvivorID, input.SourceID); err != nil {
		return MergeResult{}, false, err
	}
	var lockedCase pgtype.UUID
	if err := tx.QueryRow(ctx, `SELECT id FROM matching_cases WHERE id=$1 FOR UPDATE`, matchingUUID(input.CaseID)).Scan(&lockedCase); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return MergeResult{}, false, ErrNotFound
		}
		return MergeResult{}, false, fmt.Errorf("lock matching case for merge: %w", err)
	}
	value, err := buildMergeContext(ctx, tx, input.MergePreviewInput, now)
	if err != nil {
		return MergeResult{}, false, err
	}
	if subtle.ConstantTimeCompare(value.preview.PreviewFingerprint[:], input.PreviewFingerprint[:]) != 1 {
		return MergeResult{}, false, ErrStalePreview
	}
	if input.Confirmation != value.preview.Confirmation {
		return MergeResult{}, false, ErrInvalidConfirmation
	}
	if value.preview.UnresolvedFieldCount > 0 {
		return MergeResult{}, false, ErrInvalidInput
	}
	if len(value.preview.Conflicts) > 0 {
		return MergeResult{}, false, ErrDependencyConflict
	}
	mergedProfile, err := selectedProfile(value.preview)
	if err != nil {
		return MergeResult{}, false, err
	}
	var survivorVersion int64
	err = tx.QueryRow(ctx, `UPDATE profiles SET
  full_name=$3, social_name=$4, cpf=$5, email=$6, mobile_phone=$7, landline_phone=$8,
  address_street=$9, address_number=$10, address_complement=$11, address_neighborhood=$12,
  address_city=$13, address_state=$14, address_postal_code=$15, notes=$16,
  version=version+1, updated_at=$17
WHERE id=$1 AND version=$2
RETURNING version`, matchingUUID(input.SurvivorID), input.SurvivorVersion,
		mergedProfile.FullName, nullableString(mergedProfile.SocialName), nullableString(mergedProfile.CPF),
		nullableString(mergedProfile.Email), nullableString(mergedProfile.MobilePhone), nullableString(mergedProfile.LandlinePhone),
		nullableString(mergedProfile.AddressStreet), nullableString(mergedProfile.AddressNumber), nullableString(mergedProfile.AddressComplement),
		nullableString(mergedProfile.AddressNeighborhood), nullableString(mergedProfile.AddressCity), nullableString(mergedProfile.AddressState),
		nullableString(mergedProfile.AddressPostalCode), nullableString(mergedProfile.Notes), now).Scan(&survivorVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return MergeResult{}, false, ErrConflict
	}
	if err != nil {
		return MergeResult{}, false, normalizePostgresError(err)
	}
	if err := applyCustomFieldChoices(ctx, tx, value.customFields, input.SurvivorID, now); err != nil {
		return MergeResult{}, false, err
	}
	var unhandledCustomValues int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM custom_field_values WHERE profile_id=$1`, matchingUUID(input.SourceID)).Scan(&unhandledCustomValues); err != nil {
		return MergeResult{}, false, fmt.Errorf("verify absorbed custom Profile values: %w", err)
	}
	if unhandledCustomValues != 0 {
		return MergeResult{}, false, ErrInvalidState
	}
	if err := moveProfileDependencies(ctx, tx, input.SourceID, input.SurvivorID, now); err != nil {
		return MergeResult{}, false, err
	}
	command, err := tx.Exec(ctx, `DELETE FROM profiles WHERE id=$1 AND version=$2`, matchingUUID(input.SourceID), input.SourceVersion)
	if err != nil {
		return MergeResult{}, false, normalizePostgresError(err)
	}
	if command.RowsAffected() != 1 {
		return MergeResult{}, false, ErrConflict
	}
	if _, err := tx.Exec(ctx, `UPDATE matching_cases SET state='STALE', version=version+1, updated_at=$3
WHERE id<>$1 AND state<>'MERGED' AND (
  left_profile_id IN ($2,$4) OR right_profile_id IN ($2,$4)
)`, matchingUUID(input.CaseID), matchingUUID(input.SourceID), now, matchingUUID(input.SurvivorID)); err != nil {
		return MergeResult{}, false, fmt.Errorf("stale related matching cases: %w", err)
	}
	command, err = tx.Exec(ctx, `UPDATE matching_cases SET
  state='MERGED', decided_by_user_id=$2, decided_at=$3,
  merged_survivor_id=$4, merged_source_id=$5, merged_at=$3,
  version=version+1, updated_at=$3
WHERE id=$1 AND state='PENDING'`, matchingUUID(input.CaseID), matchingAuthUUID(actorID), now,
		matchingUUID(input.SurvivorID), matchingUUID(input.SourceID))
	if err != nil {
		return MergeResult{}, false, fmt.Errorf("mark matching case merged: %w", err)
	}
	if command.RowsAffected() != 1 {
		return MergeResult{}, false, ErrConflict
	}
	decisionID, err := NewIdentifier()
	if err != nil {
		return MergeResult{}, false, fmt.Errorf("generate merge decision identifier: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO matching_case_decisions
(id,case_id,actor_user_id,action,left_profile_version,right_profile_version,
 survivor_profile_id,source_profile_id,preview_fingerprint,request_id,decided_at)
SELECT $1,id,$2,'MERGED',left_profile_version,right_profile_version,$3,$4,$5,$6,$7
FROM matching_cases WHERE id=$8`, matchingUUID(decisionID), matchingAuthUUID(actorID),
		matchingUUID(input.SurvivorID), matchingUUID(input.SourceID), input.PreviewFingerprint[:], requestID, now, matchingUUID(input.CaseID)); err != nil {
		return MergeResult{}, false, fmt.Errorf("record profile merge decision: %w", err)
	}
	profileAuditID, err := NewIdentifier()
	if err != nil {
		return MergeResult{}, false, fmt.Errorf("generate profile merge audit identifier: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO profile_audit_events
(id,actor_user_id,profile_id,source_profile_id,event_type,outcome,request_id,occurred_at)
VALUES($1,$2,$3,$4,'PROFILE_MERGED','SUCCESS',$5,$6)`, matchingUUID(profileAuditID), matchingAuthUUID(actorID),
		matchingUUID(input.SurvivorID), matchingUUID(input.SourceID), requestID, now); err != nil {
		return MergeResult{}, false, fmt.Errorf("record profile merge audit: %w", err)
	}
	receiptID, err := NewIdentifier()
	if err != nil {
		return MergeResult{}, false, fmt.Errorf("generate merge receipt identifier: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO matching_merge_receipts
(id,actor_user_id,case_id,idempotency_key,request_fingerprint,survivor_profile_id,source_profile_id,survivor_version,merged_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, matchingUUID(receiptID), matchingAuthUUID(actorID), matchingUUID(input.CaseID),
		input.IdempotencyKey, requestFingerprint[:], matchingUUID(input.SurvivorID), matchingUUID(input.SourceID), survivorVersion, now); err != nil {
		return MergeResult{}, false, normalizePostgresError(err)
	}
	for _, dependency := range value.preview.Dependencies {
		if _, err := tx.Exec(ctx, `INSERT INTO matching_merge_receipt_counts(receipt_id,dependency_kind,affected_count)
VALUES($1,$2,$3)`, matchingUUID(receiptID), dependency.Kind, dependency.Count); err != nil {
			return MergeResult{}, false, fmt.Errorf("record merge dependency count: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return MergeResult{}, false, normalizePostgresError(err)
	}
	return MergeResult{
		CaseID: input.CaseID, SurvivorProfileID: input.SurvivorID, SourceProfileID: input.SourceID,
		SurvivorVersion: survivorVersion, MovedDependencies: value.preview.Dependencies, MergedAt: now,
	}, true, nil
}

func buildMergeContext(ctx context.Context, tx pgx.Tx, input MergePreviewInput, now time.Time) (mergeContext, error) {
	var state CaseState
	var leftID, rightID pgtype.UUID
	var leftVersion, rightVersion int64
	if err := tx.QueryRow(ctx, `SELECT state,left_profile_id,right_profile_id,left_profile_version,right_profile_version
FROM matching_cases WHERE id=$1`, matchingUUID(input.CaseID)).Scan(&state, &leftID, &rightID, &leftVersion, &rightVersion); errors.Is(err, pgx.ErrNoRows) {
		return mergeContext{}, ErrNotFound
	} else if err != nil {
		return mergeContext{}, fmt.Errorf("load matching case for preview: %w", err)
	}
	left, right := matchingIdentifier(leftID), matchingIdentifier(rightID)
	if state != CasePending {
		return mergeContext{}, ErrInvalidState
	}
	if !samePair(left, right, input.SurvivorID, input.SourceID) {
		return mergeContext{}, ErrInvalidInput
	}
	if versionForProfile(left, leftVersion, rightVersion, input.SurvivorID) != input.SurvivorVersion ||
		versionForProfile(left, leftVersion, rightVersion, input.SourceID) != input.SourceVersion {
		return mergeContext{}, ErrConflict
	}
	survivor, err := loadProfileSnapshot(ctx, tx, input.SurvivorID)
	if err != nil {
		return mergeContext{}, err
	}
	source, err := loadProfileSnapshot(ctx, tx, input.SourceID)
	if err != nil {
		return mergeContext{}, err
	}
	if survivor.Version != input.SurvivorVersion || source.Version != input.SourceVersion {
		return mergeContext{}, ErrConflict
	}
	choices := make(map[string]FieldSource, len(input.Choices))
	for _, choice := range input.Choices {
		choices[choice.FieldKey] = choice.Source
	}
	usedChoices := make(map[string]struct{}, len(choices))
	preview := MergePreview{
		CaseID: input.CaseID, Survivor: survivor, Source: source,
		Fields:       make([]MergeField, 0, MaximumCanonicalFields+16),
		Dependencies: make([]DependencyCount, 0, 8), Conflicts: make([]DependencyConflict, 0, 2),
		Confirmation: MergeConfirmation + " " + survivor.FullName, GeneratedAt: now,
	}
	for _, definition := range canonicalMergeFields {
		field := mergeField(definition.key, definition.label, definition.kind, definition.value(survivor), definition.value(source), choices)
		if field.Conflict {
			if _, exists := choices[field.Key]; exists {
				usedChoices[field.Key] = struct{}{}
			}
		}
		if field.ChoiceRequired && !field.SelectedSource.Valid() {
			preview.UnresolvedFieldCount++
		}
		preview.Fields = append(preview.Fields, field)
	}
	customFields, err := loadCustomMergeFields(ctx, tx, input.SurvivorID, input.SourceID, choices)
	if err != nil {
		return mergeContext{}, err
	}
	for _, custom := range customFields {
		if custom.field.Conflict {
			if _, exists := choices[custom.field.Key]; exists {
				usedChoices[custom.field.Key] = struct{}{}
			}
		}
		if custom.field.ChoiceRequired && !custom.field.SelectedSource.Valid() {
			preview.UnresolvedFieldCount++
		}
		preview.Fields = append(preview.Fields, custom.field)
	}
	if len(usedChoices) != len(choices) {
		return mergeContext{}, ErrInvalidInput
	}
	dependencies, conflicts, digest, err := loadMergeDependencies(ctx, tx, input.SurvivorID, input.SourceID)
	if err != nil {
		return mergeContext{}, err
	}
	preview.Dependencies, preview.Conflicts = dependencies, conflicts
	fingerprint, err := previewDigest(preview, digest)
	if err != nil {
		return mergeContext{}, err
	}
	preview.PreviewFingerprint = fingerprint
	return mergeContext{preview: preview, customFields: customFields, dependencyDigest: digest}, nil
}

func loadProfileSnapshot(ctx context.Context, tx pgx.Tx, id Identifier) (ProfileSnapshot, error) {
	var value ProfileSnapshot
	var databaseID pgtype.UUID
	err := tx.QueryRow(ctx, `SELECT id,full_name,COALESCE(social_name,''),COALESCE(cpf,''),COALESCE(email,''),
       COALESCE(mobile_phone,''),COALESCE(landline_phone,''),COALESCE(address_street,''),COALESCE(address_number,''),
       COALESCE(address_complement,''),COALESCE(address_neighborhood,''),COALESCE(address_city,''),
       COALESCE(address_state,''),COALESCE(address_postal_code,''),COALESCE(notes,''),version,updated_at
FROM profiles WHERE id=$1`, matchingUUID(id)).Scan(&databaseID, &value.FullName, &value.SocialName, &value.CPF,
		&value.Email, &value.MobilePhone, &value.LandlinePhone, &value.AddressStreet, &value.AddressNumber,
		&value.AddressComplement, &value.AddressNeighborhood, &value.AddressCity, &value.AddressState,
		&value.AddressPostalCode, &value.Notes, &value.Version, &value.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ProfileSnapshot{}, ErrNotFound
	}
	if err != nil {
		return ProfileSnapshot{}, fmt.Errorf("load merge profile: %w", err)
	}
	value.ID = matchingIdentifier(databaseID)
	return value, nil
}

func loadCustomMergeFields(ctx context.Context, tx pgx.Tx, survivorID, sourceID Identifier, choices map[string]FieldSource) ([]customMergeField, error) {
	rows, err := tx.Query(ctx, `SELECT definition.id::text, definition.label, definition.field_kind,
       survivor_value.id, CASE
         WHEN survivor_value.id IS NULL THEN NULL
         WHEN definition.field_kind IN ('TEXT','LONG_TEXT','EMAIL','PHONE') THEN survivor_value.text_value
         WHEN definition.field_kind='INTEGER' THEN survivor_value.integer_value::text
         WHEN definition.field_kind='DECIMAL' THEN survivor_value.decimal_value::text
         WHEN definition.field_kind='BOOLEAN' THEN survivor_value.boolean_value::text
         WHEN definition.field_kind='CIVIL_DATE' THEN survivor_value.civil_date_value::text
         WHEN definition.field_kind='CIVIL_MONTH' THEN survivor_value.civil_month_value
         ELSE (SELECT COALESCE(string_agg(option.technical_key,',' ORDER BY option.technical_key),'')
               FROM custom_field_value_options selected
               JOIN custom_field_options option ON option.id=selected.option_id
               WHERE selected.custom_field_value_id=survivor_value.id)
       END,
       source_value.id, CASE
         WHEN source_value.id IS NULL THEN NULL
         WHEN definition.field_kind IN ('TEXT','LONG_TEXT','EMAIL','PHONE') THEN source_value.text_value
         WHEN definition.field_kind='INTEGER' THEN source_value.integer_value::text
         WHEN definition.field_kind='DECIMAL' THEN source_value.decimal_value::text
         WHEN definition.field_kind='BOOLEAN' THEN source_value.boolean_value::text
         WHEN definition.field_kind='CIVIL_DATE' THEN source_value.civil_date_value::text
         WHEN definition.field_kind='CIVIL_MONTH' THEN source_value.civil_month_value
         ELSE (SELECT COALESCE(string_agg(option.technical_key,',' ORDER BY option.technical_key),'')
               FROM custom_field_value_options selected
               JOIN custom_field_options option ON option.id=selected.option_id
               WHERE selected.custom_field_value_id=source_value.id)
       END
FROM custom_field_definitions definition
LEFT JOIN custom_field_values survivor_value ON survivor_value.field_definition_id=definition.id AND survivor_value.profile_id=$1
LEFT JOIN custom_field_values source_value ON source_value.field_definition_id=definition.id AND source_value.profile_id=$2
WHERE definition.target_kind='PROFILE' AND definition.field_kind<>'ATTACHMENT'
ORDER BY definition.technical_key, definition.id
LIMIT 257`, matchingUUID(survivorID), matchingUUID(sourceID))
	if err != nil {
		return nil, fmt.Errorf("load custom Profile fields for merge: %w", err)
	}
	defer rows.Close()
	result := make([]customMergeField, 0)
	for rows.Next() {
		var definitionText, label, kind string
		var survivorValueID, sourceValueID pgtype.UUID
		var survivorValue, sourceValue pgtype.Text
		if err := rows.Scan(&definitionText, &label, &kind, &survivorValueID, &survivorValue, &sourceValueID, &sourceValue); err != nil {
			return nil, fmt.Errorf("scan custom Profile merge field: %w", err)
		}
		definitionID, err := ParseIdentifier(definitionText)
		if err != nil {
			return nil, ErrInvalidState
		}
		value := customMergeField{definitionID: definitionID}
		if survivorValueID.Valid {
			identifier := matchingIdentifier(survivorValueID)
			value.survivorValueID = &identifier
		}
		if sourceValueID.Valid {
			identifier := matchingIdentifier(sourceValueID)
			value.sourceValueID = &identifier
		}
		value.field = mergeField("custom."+definitionText, label, kind, nullableTextPointer(survivorValue), nullableTextPointer(sourceValue), choices)
		result = append(result, value)
		if len(result) > MaximumCustomFields {
			return nil, ErrDependencyConflict
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate custom Profile merge fields: %w", err)
	}
	return result, nil
}

func loadMergeDependencies(ctx context.Context, tx pgx.Tx, survivorID, sourceID Identifier) ([]DependencyCount, []DependencyConflict, []string, error) {
	var counts [8]int
	var digests [8]string
	err := tx.QueryRow(ctx, `SELECT
  (SELECT count(*) FROM documents WHERE owner_profile_id=$1),
  (SELECT count(*) FROM document_current_uses WHERE holder_profile_id=$1),
  (SELECT count(*) FROM bills WHERE owner_profile_id=$1),
  (SELECT count(*) FROM bill_current_uses WHERE holder_profile_id=$1),
  (SELECT count(*) FROM custom_entities WHERE owner_profile_id=$1),
  (SELECT count(*) FROM custom_field_values WHERE profile_id=$1),
  (SELECT count(*) FROM attachment_upload_intents WHERE custom_profile_id=$1),
  (SELECT count(*) FROM attachments WHERE custom_profile_id=$1),
  (SELECT md5(COALESCE(string_agg(id::text||':'||version::text,',' ORDER BY id),'')) FROM documents WHERE owner_profile_id=$1),
  (SELECT md5(COALESCE(string_agg(document_id::text,',' ORDER BY document_id),'')) FROM document_current_uses WHERE holder_profile_id=$1),
  (SELECT md5(COALESCE(string_agg(id::text||':'||version::text,',' ORDER BY id),'')) FROM bills WHERE owner_profile_id=$1),
  (SELECT md5(COALESCE(string_agg(bill_id::text,',' ORDER BY bill_id),'')) FROM bill_current_uses WHERE holder_profile_id=$1),
  (SELECT md5(COALESCE(string_agg(id::text||':'||version::text,',' ORDER BY id),'')) FROM custom_entities WHERE owner_profile_id=$1),
  (SELECT md5(COALESCE(string_agg(id::text||':'||version::text,',' ORDER BY id),'')) FROM custom_field_values WHERE profile_id=$1),
  (SELECT md5(COALESCE(string_agg(id::text,',' ORDER BY id),'')) FROM attachment_upload_intents WHERE custom_profile_id=$1),
  (SELECT md5(COALESCE(string_agg(id::text||':'||version::text,',' ORDER BY id),'')) FROM attachments WHERE custom_profile_id=$1)`, matchingUUID(sourceID)).Scan(
		&counts[0], &counts[1], &counts[2], &counts[3], &counts[4], &counts[5], &counts[6], &counts[7],
		&digests[0], &digests[1], &digests[2], &digests[3], &digests[4], &digests[5], &digests[6], &digests[7])
	if err != nil {
		return nil, nil, nil, fmt.Errorf("load merge dependency topology: %w", err)
	}
	kinds := []string{
		DependencyDocumentOwner, DependencyDocumentHolder, DependencyBillOwner, DependencyBillHolder,
		DependencyCustomEntity, DependencyCustomValue, DependencyAttachmentIntent, DependencyAttachment,
	}
	dependencies := make([]DependencyCount, len(kinds))
	for index, kind := range kinds {
		dependencies[index] = DependencyCount{Kind: kind, Count: counts[index]}
	}
	var documentConflicts, entityConflicts int
	if err := tx.QueryRow(ctx, `SELECT
  (SELECT count(*) FROM documents source_document
   JOIN documents survivor_document ON survivor_document.owner_profile_id=$2
    AND survivor_document.document_type_id=source_document.document_type_id
    AND survivor_document.identifier_value=source_document.identifier_value
    AND survivor_document.uniqueness_policy='PER_PROFILE'
   WHERE source_document.owner_profile_id=$1 AND source_document.uniqueness_policy='PER_PROFILE'),
  (SELECT count(*) FROM custom_entities source_entity
   JOIN custom_entities survivor_entity ON survivor_entity.owner_profile_id=$2
    AND survivor_entity.custom_entity_type_id=source_entity.custom_entity_type_id
    AND survivor_entity.profile_cardinality='ONE_PER_PROFILE'
   WHERE source_entity.owner_profile_id=$1 AND source_entity.profile_cardinality='ONE_PER_PROFILE')`,
		matchingUUID(sourceID), matchingUUID(survivorID)).Scan(&documentConflicts, &entityConflicts); err != nil {
		return nil, nil, nil, fmt.Errorf("load merge dependency conflicts: %w", err)
	}
	conflicts := make([]DependencyConflict, 0, 2)
	if documentConflicts > 0 {
		conflicts = append(conflicts, DependencyConflict{Kind: ConflictDocumentUnique, Count: documentConflicts})
	}
	if entityConflicts > 0 {
		conflicts = append(conflicts, DependencyConflict{Kind: ConflictCustomEntity, Count: entityConflicts})
	}
	return dependencies, conflicts, digests[:], nil
}

func applyCustomFieldChoices(ctx context.Context, tx pgx.Tx, fields []customMergeField, survivorID Identifier, now time.Time) error {
	for _, value := range fields {
		choice := value.field.SelectedSource
		if !value.field.Conflict {
			choice = FieldFromSurvivor
			if value.survivorValueID == nil && value.sourceValueID != nil {
				choice = FieldFromSource
			}
		}
		switch choice {
		case FieldFromSurvivor:
			if value.sourceValueID != nil {
				if _, err := tx.Exec(ctx, `DELETE FROM custom_field_values WHERE id=$1`, matchingUUID(*value.sourceValueID)); err != nil {
					return fmt.Errorf("discard absorbed custom Profile value: %w", err)
				}
			}
		case FieldFromSource:
			if value.survivorValueID != nil {
				if _, err := tx.Exec(ctx, `DELETE FROM custom_field_values WHERE id=$1`, matchingUUID(*value.survivorValueID)); err != nil {
					return fmt.Errorf("replace survivor custom Profile value: %w", err)
				}
			}
			if value.sourceValueID != nil {
				if _, err := tx.Exec(ctx, `UPDATE custom_field_values
SET profile_id=$2, version=version+1, updated_at=$3 WHERE id=$1`, matchingUUID(*value.sourceValueID), matchingUUID(survivorID), now); err != nil {
					return fmt.Errorf("move selected custom Profile value: %w", err)
				}
			}
		default:
			return ErrInvalidInput
		}
	}
	return nil
}

func moveProfileDependencies(ctx context.Context, tx pgx.Tx, sourceID, survivorID Identifier, now time.Time) error {
	statements := []struct {
		query string
		name  string
	}{
		{`UPDATE documents SET owner_profile_id=$2,version=version+1,updated_at=$3 WHERE owner_profile_id=$1`, "document owners"},
		{`UPDATE document_current_uses SET holder_profile_id=$2 WHERE holder_profile_id=$1`, "document holders"},
		{`UPDATE bills SET owner_profile_id=$2,version=version+1,updated_at=$3 WHERE owner_profile_id=$1`, "bill owners"},
		{`UPDATE bill_current_uses SET holder_profile_id=$2 WHERE holder_profile_id=$1`, "bill holders"},
		{`UPDATE custom_entities SET owner_profile_id=$2,version=version+1,updated_at=$3 WHERE owner_profile_id=$1`, "custom entity owners"},
		{`UPDATE attachment_upload_intents SET custom_profile_id=$2 WHERE custom_profile_id=$1`, "attachment intents"},
		{`UPDATE attachments SET custom_profile_id=$2,version=version+1,updated_at=$3 WHERE custom_profile_id=$1`, "attachments"},
	}
	for _, statement := range statements {
		if _, err := tx.Exec(ctx, statement.query, matchingUUID(sourceID), matchingUUID(survivorID), now); err != nil {
			return fmt.Errorf("move merged Profile %s: %w", statement.name, normalizePostgresError(err))
		}
	}
	return nil
}

func selectedProfile(preview MergePreview) (ProfileSnapshot, error) {
	result := preview.Survivor
	for _, field := range preview.Fields {
		if strings.HasPrefix(field.Key, "custom.") {
			continue
		}
		value := field.SurvivorValue
		if field.Conflict {
			switch field.SelectedSource {
			case FieldFromSurvivor:
			case FieldFromSource:
				value = field.SourceValue
			default:
				return ProfileSnapshot{}, ErrInvalidInput
			}
		}
		if err := setProfileField(&result, field.Key, value); err != nil {
			return ProfileSnapshot{}, err
		}
	}
	return result, nil
}

func setProfileField(value *ProfileSnapshot, key string, selected *string) error {
	text := ""
	if selected != nil {
		text = *selected
	}
	switch key {
	case "full_name":
		if text == "" {
			return ErrInvalidInput
		}
		value.FullName = text
	case "social_name":
		value.SocialName = text
	case "cpf":
		value.CPF = text
	case "email":
		value.Email = text
	case "mobile_phone":
		value.MobilePhone = text
	case "landline_phone":
		value.LandlinePhone = text
	case "address_street":
		value.AddressStreet = text
	case "address_number":
		value.AddressNumber = text
	case "address_complement":
		value.AddressComplement = text
	case "address_neighborhood":
		value.AddressNeighborhood = text
	case "address_city":
		value.AddressCity = text
	case "address_state":
		value.AddressState = text
	case "address_postal_code":
		value.AddressPostalCode = text
	case "notes":
		value.Notes = text
	default:
		return ErrInvalidInput
	}
	return nil
}

func mergeField(key, label, kind string, survivorValue, sourceValue *string, choices map[string]FieldSource) MergeField {
	conflict := !equalOptionalStrings(survivorValue, sourceValue)
	field := MergeField{
		Key: key, Label: label, Kind: kind, SurvivorValue: survivorValue, SourceValue: sourceValue,
		Conflict: conflict, ChoiceRequired: conflict,
	}
	if conflict {
		field.SelectedSource = choices[key]
	}
	return field
}

func previewDigest(preview MergePreview, dependencyDigest []string) ([sha256.Size]byte, error) {
	type digestPayload struct {
		CaseID           string
		SurvivorID       string
		SourceID         string
		SurvivorVersion  int64
		SourceVersion    int64
		Fields           []MergeField
		Dependencies     []DependencyCount
		Conflicts        []DependencyConflict
		DependencyDigest []string
	}
	payload, err := json.Marshal(digestPayload{
		CaseID: preview.CaseID.String(), SurvivorID: preview.Survivor.ID.String(), SourceID: preview.Source.ID.String(),
		SurvivorVersion: preview.Survivor.Version, SourceVersion: preview.Source.Version,
		Fields: preview.Fields, Dependencies: preview.Dependencies, Conflicts: preview.Conflicts,
		DependencyDigest: dependencyDigest,
	})
	if err != nil {
		return [sha256.Size]byte{}, fmt.Errorf("encode merge preview fingerprint: %w", err)
	}
	return sha256.Sum256(payload), nil
}

func mergeInputFingerprint(input MergeInput) ([sha256.Size]byte, error) {
	choices := append([]FieldChoice(nil), input.Choices...)
	sort.Slice(choices, func(left, right int) bool { return choices[left].FieldKey < choices[right].FieldKey })
	payload, err := json.Marshal(struct {
		CaseID, SurvivorID, SourceID   string
		SurvivorVersion, SourceVersion int64
		Choices                        []FieldChoice
		PreviewFingerprint             string
		Confirmation                   string
	}{
		CaseID: input.CaseID.String(), SurvivorID: input.SurvivorID.String(), SourceID: input.SourceID.String(),
		SurvivorVersion: input.SurvivorVersion, SourceVersion: input.SourceVersion, Choices: choices,
		PreviewFingerprint: fmt.Sprintf("%x", input.PreviewFingerprint[:]),
		Confirmation:       input.Confirmation,
	})
	if err != nil {
		return [sha256.Size]byte{}, fmt.Errorf("encode merge request fingerprint: %w", err)
	}
	return sha256.Sum256(payload), nil
}

func loadMergeReceipt(ctx context.Context, tx pgx.Tx, actorID auth.Identifier, key string, expected [sha256.Size]byte) (MergeResult, bool, error) {
	var receiptID, caseID, survivorID, sourceID pgtype.UUID
	var fingerprint []byte
	var version int64
	var mergedAt time.Time
	err := tx.QueryRow(ctx, `SELECT id,case_id,request_fingerprint,survivor_profile_id,source_profile_id,survivor_version,merged_at
FROM matching_merge_receipts WHERE actor_user_id=$1 AND idempotency_key=$2 FOR UPDATE`, matchingAuthUUID(actorID), key).Scan(
		&receiptID, &caseID, &fingerprint, &survivorID, &sourceID, &version, &mergedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return MergeResult{}, false, nil
	}
	if err != nil {
		return MergeResult{}, false, fmt.Errorf("load profile merge receipt: %w", err)
	}
	if len(fingerprint) != sha256.Size || subtle.ConstantTimeCompare(fingerprint, expected[:]) != 1 {
		return MergeResult{}, false, ErrConflict
	}
	result := MergeResult{
		CaseID: matchingIdentifier(caseID), SurvivorProfileID: matchingIdentifier(survivorID), SourceProfileID: matchingIdentifier(sourceID),
		SurvivorVersion: version, MergedAt: mergedAt, MovedDependencies: make([]DependencyCount, 0, 8),
	}
	rows, err := tx.Query(ctx, `SELECT dependency_kind,affected_count FROM matching_merge_receipt_counts
WHERE receipt_id=$1 ORDER BY dependency_kind`, receiptID)
	if err != nil {
		return MergeResult{}, false, fmt.Errorf("load merge receipt counts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var value DependencyCount
		if err := rows.Scan(&value.Kind, &value.Count); err != nil {
			return MergeResult{}, false, fmt.Errorf("scan merge receipt count: %w", err)
		}
		result.MovedDependencies = append(result.MovedDependencies, value)
	}
	if err := rows.Err(); err != nil {
		return MergeResult{}, false, fmt.Errorf("iterate merge receipt counts: %w", err)
	}
	return result, true, nil
}

func lockProfilesInOrder(ctx context.Context, tx pgx.Tx, first, second Identifier) error {
	rows, err := tx.Query(ctx, `SELECT id FROM profiles WHERE id IN ($1,$2) ORDER BY id FOR UPDATE`, matchingUUID(first), matchingUUID(second))
	if err != nil {
		return fmt.Errorf("lock Profiles for merge: %w", err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var id pgtype.UUID
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("scan locked merge Profile: %w", err)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate locked merge Profiles: %w", err)
	}
	if count != 2 {
		return ErrNotFound
	}
	return nil
}

func samePair(left, right, first, second Identifier) bool {
	return (left == first && right == second) || (left == second && right == first)
}

func versionForProfile(left Identifier, leftVersion, rightVersion int64, id Identifier) int64 {
	if id == left {
		return leftVersion
	}
	return rightVersion
}

func requiredString(value string) *string {
	copyValue := value
	return &copyValue
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	copyValue := value
	return &copyValue
}

func nullableTextPointer(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	copyValue := value.String
	return &copyValue
}

func equalOptionalStrings(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
