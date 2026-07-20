package aichat

import (
	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5/pgtype"
)

func scanRun(row rowScanner) (Run, error) {
	var value Run
	var id, thread, owner, retry pgtype.UUID
	var fingerprint []byte
	var errorCode pgtype.Text
	var cancel, started, completed pgtype.Timestamptz
	if err := row.Scan(&id, &thread, &owner, &retry, &value.IdempotencyKey, &fingerprint, &value.State,
		&value.ToolCallCount, &value.InputUsage, &value.OutputUsage, &value.ResultBytes, &errorCode,
		&cancel, &started, &completed, &value.Version, &value.CreatedAt, &value.UpdatedAt); err != nil {
		return Run{}, err
	}
	if len(fingerprint) != 32 {
		return Run{}, ErrInvalidState
	}
	value.ID, value.ThreadID, value.OwnerUserID = chatIdentifier(id), chatIdentifier(thread), auth.Identifier(owner.Bytes)
	value.RetryOfRunID = optionalIdentifier(retry)
	copy(value.RequestFingerprint[:], fingerprint)
	if errorCode.Valid {
		value.ErrorCode = errorCode.String
	}
	if cancel.Valid {
		value.CancelRequestedAt = &cancel.Time
	}
	if started.Valid {
		value.StartedAt = &started.Time
	}
	if completed.Valid {
		value.CompletedAt = &completed.Time
	}
	return value, nil
}

func scanToolStep(row rowScanner) (ToolStep, error) {
	var value ToolStep
	var id, run, reference pgtype.UUID
	var fingerprint []byte
	var errorCode pgtype.Text
	var completed pgtype.Timestamptz
	if err := row.Scan(&id, &run, &value.Sequence, &value.Kind, &value.State, &fingerprint, &reference,
		&value.RowCount, &value.ResultBytes, &errorCode, &value.StartedAt, &completed); err != nil {
		return ToolStep{}, err
	}
	if len(fingerprint) != 32 {
		return ToolStep{}, ErrInvalidState
	}
	value.ID, value.RunID = chatIdentifier(id), chatIdentifier(run)
	copy(value.ArgumentsFingerprint[:], fingerprint)
	value.ResultReferenceID = optionalIdentifier(reference)
	if errorCode.Valid {
		value.ErrorCode = errorCode.String
	}
	if completed.Valid {
		value.CompletedAt = &completed.Time
	}
	return value, nil
}

func scanResultReference(row rowScanner) (ResultReference, error) {
	var value ResultReference
	var id, thread, run, owner, execution pgtype.UUID
	var fingerprint []byte
	if err := row.Scan(&id, &thread, &run, &owner, &value.Kind, &execution, &value.LogicalRequest, &fingerprint,
		&value.Label, &value.RowCount, &value.ColumnCount, &value.ExpiresAt, &value.CreatedAt); err != nil {
		return ResultReference{}, err
	}
	if len(fingerprint) != 32 {
		return ResultReference{}, ErrInvalidState
	}
	value.ID, value.ThreadID, value.RunID = chatIdentifier(id), chatIdentifier(thread), chatIdentifier(run)
	value.OwnerUserID, value.QueryExecutionID = auth.Identifier(owner.Bytes), optionalIdentifier(execution)
	copy(value.ContextFingerprint[:], fingerprint)
	return value, nil
}

func scanRunEvent(row rowScanner) (RunEvent, error) {
	var value RunEvent
	var run, tool, reference pgtype.UUID
	var text, errorCode pgtype.Text
	if err := row.Scan(&run, &value.Sequence, &value.Kind, &text, &tool, &reference, &errorCode, &value.CreatedAt); err != nil {
		return RunEvent{}, err
	}
	value.RunID, value.ToolStepID, value.ResultReferenceID = chatIdentifier(run), optionalIdentifier(tool), optionalIdentifier(reference)
	if text.Valid {
		value.TextDelta = text.String
	}
	if errorCode.Valid {
		value.ErrorCode = errorCode.String
	}
	return value, nil
}
