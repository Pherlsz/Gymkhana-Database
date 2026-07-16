CREATE TABLE document_audit_events (
  id UUID PRIMARY KEY,
  actor_user_id UUID REFERENCES app_users(id) ON DELETE SET NULL,
  document_id UUID,
  source_document_id UUID,
  document_type_id UUID,
  holder_profile_id UUID,
  event_type TEXT NOT NULL CHECK (
    event_type IN (
      'DOCUMENT_TYPE_CREATED', 'DOCUMENT_TYPE_UPDATED', 'DOCUMENT_TYPE_DELETED',
      'DOCUMENT_CREATED', 'DOCUMENT_UPDATED', 'DOCUMENT_DUPLICATED', 'DOCUMENT_DELETED',
      'DOCUMENT_USE_ASSIGNED', 'DOCUMENT_USE_RETURNED'
    )
  ),
  outcome TEXT NOT NULL CHECK (outcome IN ('SUCCESS', 'DENIED', 'FAILURE')),
  request_id TEXT NOT NULL CHECK (request_id <> '' AND char_length(request_id) <= 128),
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (document_id IS NOT NULL OR document_type_id IS NOT NULL)
);
CREATE INDEX document_audit_events_occurred_at_index ON document_audit_events (occurred_at DESC);
CREATE INDEX document_audit_events_actor_index ON document_audit_events (actor_user_id, occurred_at DESC);
CREATE INDEX document_audit_events_document_index ON document_audit_events (document_id, occurred_at DESC) WHERE document_id IS NOT NULL;
CREATE INDEX document_audit_events_type_index ON document_audit_events (document_type_id, occurred_at DESC) WHERE document_type_id IS NOT NULL;
