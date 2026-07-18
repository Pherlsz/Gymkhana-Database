package ocr

import (
	"fmt"

	"github.com/Pherlsz/Gymkhana-Database/internal/attachment"
	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5/pgtype"
)

const jobColumns = `id,owner_user_id,attachment_id,retry_of_job_id,idempotency_key,request_fingerprint,
source_sha256,catalog_fingerprint,schema_version,source_mime,source_bytes,state,river_job_id,
attempt_count,page_count,pixel_count,suggestion_count,provider_usage,error_code,cancel_requested_at,started_at,
completed_at,version,created_at,updated_at`

type scanner interface{ Scan(...any) error }

func scanJob(row scanner) (Job, error) {
	var value Job
	var id, owner, source pgtype.UUID
	var retry pgtype.UUID
	var requestFingerprint, sourceSHA, catalogFingerprint []byte
	var river pgtype.Int8
	var errorCode pgtype.Text
	err := row.Scan(
		&id, &owner, &source, &retry, &value.IdempotencyKey, &requestFingerprint,
		&sourceSHA, &catalogFingerprint, &value.SchemaVersion, &value.SourceMIME, &value.SourceBytes,
		&value.State, &river, &value.AttemptCount, &value.PageCount, &value.PixelCount, &value.SuggestionCount,
		&value.ProviderUsage, &errorCode, &value.CancelRequestedAt, &value.StartedAt,
		&value.CompletedAt, &value.Version, &value.CreatedAt, &value.UpdatedAt,
	)
	if err != nil {
		return Job{}, err
	}
	if !id.Valid || !owner.Valid || !source.Valid || len(requestFingerprint) != 32 || len(sourceSHA) != 32 || len(catalogFingerprint) != 32 {
		return Job{}, fmt.Errorf("scan OCR job: %w", ErrInvalidState)
	}
	value.ID = Identifier(id.Bytes)
	value.OwnerUserID = auth.Identifier(owner.Bytes)
	value.AttachmentID = attachment.Identifier(source.Bytes)
	copy(value.RequestFingerprint[:], requestFingerprint)
	copy(value.SourceSHA256[:], sourceSHA)
	copy(value.CatalogFingerprint[:], catalogFingerprint)
	if retry.Valid {
		mapped := Identifier(retry.Bytes)
		value.RetryOfJobID = &mapped
	}
	if river.Valid {
		value.RiverJobID = river.Int64
	}
	if errorCode.Valid {
		value.ErrorCode = errorCode.String
	}
	return value, nil
}

const suggestionColumns = `id,job_id,ordinal,target_kind,target_id,target_version,field_key,field_label,value_kind,
proposed_value,confidence,evidence_page,region_x,region_y,region_width,region_height,evidence_excerpt,
review_state,reviewed_value,reviewed_by_user_id,reviewed_at,applied_at,version,created_at,updated_at`

func scanSuggestion(row scanner) (Suggestion, error) {
	var value Suggestion
	var id, jobID, targetID, reviewer pgtype.UUID
	var confidence pgtype.Int2
	var regionX, regionY, regionWidth, regionHeight pgtype.Int4
	var excerpt, reviewed pgtype.Text
	err := row.Scan(
		&id, &jobID, &value.Ordinal, &value.Target.Kind, &targetID, &value.TargetVersion,
		&value.FieldKey, &value.FieldLabel, &value.Kind, &value.ProposedValue, &confidence,
		&value.Evidence.Page, &regionX, &regionY, &regionWidth, &regionHeight, &excerpt,
		&value.ReviewState, &reviewed, &reviewer, &value.ReviewedAt, &value.AppliedAt,
		&value.Version, &value.CreatedAt, &value.UpdatedAt,
	)
	if err != nil {
		return Suggestion{}, err
	}
	if !id.Valid || !jobID.Valid || !targetID.Valid {
		return Suggestion{}, fmt.Errorf("scan OCR suggestion: %w", ErrInvalidState)
	}
	value.ID = Identifier(id.Bytes)
	value.JobID = Identifier(jobID.Bytes)
	value.Target.ID = Identifier(targetID.Bytes)
	if confidence.Valid {
		mapped := int(confidence.Int16)
		value.Evidence.Confidence = &mapped
	}
	if regionX.Valid || regionY.Valid || regionWidth.Valid || regionHeight.Valid {
		if !regionX.Valid || !regionY.Valid || !regionWidth.Valid || !regionHeight.Valid {
			return Suggestion{}, fmt.Errorf("scan OCR evidence region: %w", ErrInvalidState)
		}
		value.Evidence.Region = &Region{
			X: int(regionX.Int32), Y: int(regionY.Int32), Width: int(regionWidth.Int32), Height: int(regionHeight.Int32),
		}
	}
	if excerpt.Valid {
		value.Evidence.Excerpt = excerpt.String
	}
	if reviewed.Valid {
		mapped := reviewed.String
		value.ReviewedValue = &mapped
	}
	if reviewer.Valid {
		mapped := auth.Identifier(reviewer.Bytes)
		value.ReviewedByUserID = &mapped
	}
	return value, nil
}

const applyReceiptColumns = `id,job_id,owner_user_id,idempotency_key,request_fingerprint,state,created_at,updated_at,completed_at`

func scanApplyReceipt(row scanner) (ApplyReceipt, error) {
	var value ApplyReceipt
	var id, jobID, owner pgtype.UUID
	var fingerprint []byte
	if err := row.Scan(&id, &jobID, &owner, &value.IdempotencyKey, &fingerprint, &value.State, &value.CreatedAt, &value.UpdatedAt, &value.CompletedAt); err != nil {
		return ApplyReceipt{}, err
	}
	if !id.Valid || !jobID.Valid || !owner.Valid || len(fingerprint) != 32 {
		return ApplyReceipt{}, fmt.Errorf("scan OCR apply receipt: %w", ErrInvalidState)
	}
	value.ID = Identifier(id.Bytes)
	value.JobID = Identifier(jobID.Bytes)
	value.OwnerUserID = auth.Identifier(owner.Bytes)
	copy(value.RequestFingerprint[:], fingerprint)
	return value, nil
}

func scanApplyResult(row scanner) (ApplyResult, error) {
	var value ApplyResult
	var receiptID, suggestionID pgtype.UUID
	var targetVersion pgtype.Int8
	var errorCode pgtype.Text
	if err := row.Scan(&receiptID, &suggestionID, &value.Outcome, &targetVersion, &errorCode, &value.CreatedAt); err != nil {
		return ApplyResult{}, err
	}
	if !receiptID.Valid || !suggestionID.Valid {
		return ApplyResult{}, fmt.Errorf("scan OCR apply result: %w", ErrInvalidState)
	}
	value.ReceiptID = Identifier(receiptID.Bytes)
	value.SuggestionID = Identifier(suggestionID.Bytes)
	if targetVersion.Valid {
		mapped := targetVersion.Int64
		value.TargetVersion = &mapped
	}
	if errorCode.Valid {
		value.ErrorCode = errorCode.String
	}
	return value, nil
}

func databaseUUID(value Identifier) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(value), Valid: !value.IsZero()}
}

func authDatabaseUUID(value auth.Identifier) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(value), Valid: value != (auth.Identifier{})}
}

func attachmentDatabaseUUID(value attachment.Identifier) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(value), Valid: !value.IsZero()}
}

func optionalDatabaseUUID(value *Identifier) pgtype.UUID {
	if value == nil {
		return pgtype.UUID{}
	}
	return databaseUUID(*value)
}

func optionalInt(value *int) pgtype.Int4 {
	if value == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*value), Valid: true}
}

func optionalRegion(value *Region) (pgtype.Int4, pgtype.Int4, pgtype.Int4, pgtype.Int4) {
	if value == nil {
		return pgtype.Int4{}, pgtype.Int4{}, pgtype.Int4{}, pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(value.X), Valid: true}, pgtype.Int4{Int32: int32(value.Y), Valid: true},
		pgtype.Int4{Int32: int32(value.Width), Valid: true}, pgtype.Int4{Int32: int32(value.Height), Valid: true}
}
