-- name: RecordBillAuditEvent :exec
INSERT INTO bill_audit_events (
  id, actor_user_id, bill_id, source_bill_id, bill_type_id,
  holder_profile_id, event_type, outcome, request_id
) VALUES (
  sqlc.arg(id), sqlc.arg(actor_user_id), sqlc.narg(bill_id),
  sqlc.narg(source_bill_id), sqlc.narg(bill_type_id),
  sqlc.narg(holder_profile_id), sqlc.arg(event_type), sqlc.arg(outcome), sqlc.arg(request_id)
);
