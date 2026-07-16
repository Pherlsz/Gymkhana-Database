CREATE TABLE bill_audit_events (
  id UUID PRIMARY KEY,
  actor_user_id UUID REFERENCES app_users(id) ON DELETE SET NULL,
  bill_id UUID,
  source_bill_id UUID,
  bill_type_id UUID,
  holder_profile_id UUID,
  event_type TEXT NOT NULL CHECK (
    event_type IN (
      'BILL_TYPE_CREATED', 'BILL_TYPE_UPDATED', 'BILL_TYPE_DELETED',
      'BILL_CREATED', 'BILL_UPDATED', 'BILL_DUPLICATED', 'BILL_DELETED',
      'BILL_USE_ASSIGNED', 'BILL_USE_RETURNED'
    )
  ),
  outcome TEXT NOT NULL CHECK (outcome IN ('SUCCESS', 'DENIED', 'FAILURE')),
  request_id TEXT NOT NULL CHECK (request_id <> '' AND char_length(request_id) <= 128),
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (bill_id IS NOT NULL OR bill_type_id IS NOT NULL)
);
CREATE INDEX bill_audit_events_occurred_at_index ON bill_audit_events (occurred_at DESC);
CREATE INDEX bill_audit_events_actor_index ON bill_audit_events (actor_user_id, occurred_at DESC);
CREATE INDEX bill_audit_events_bill_index ON bill_audit_events (bill_id, occurred_at DESC) WHERE bill_id IS NOT NULL;
CREATE INDEX bill_audit_events_type_index ON bill_audit_events (bill_type_id, occurred_at DESC) WHERE bill_type_id IS NOT NULL;
