-- name: RecordDocumentAuditEvent :exec
INSERT INTO document_audit_events (
  id, actor_user_id, document_id, source_document_id, document_type_id,
  holder_profile_id, event_type, outcome, request_id
) VALUES (
  sqlc.arg(id), sqlc.arg(actor_user_id), sqlc.narg(document_id),
  sqlc.narg(source_document_id), sqlc.narg(document_type_id),
  sqlc.narg(holder_profile_id), sqlc.arg(event_type), sqlc.arg(outcome), sqlc.arg(request_id)
);
