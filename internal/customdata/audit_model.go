package customdata

import (
	"strings"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

type AuditResourceKind string

const (
	AuditResourceEntityType      AuditResourceKind = "ENTITY_TYPE"
	AuditResourceFieldDefinition AuditResourceKind = "FIELD_DEFINITION"
	AuditResourceFieldOption     AuditResourceKind = "FIELD_OPTION"
	AuditResourceCustomEntity    AuditResourceKind = "CUSTOM_ENTITY"
	AuditResourceFieldValues     AuditResourceKind = "FIELD_VALUES"
)

type AuditEventType string

const (
	AuditEventEntityTypeCreated AuditEventType = "CUSTOM_ENTITY_TYPE_CREATED"
	AuditEventEntityTypeUpdated AuditEventType = "CUSTOM_ENTITY_TYPE_UPDATED"
	AuditEventEntityTypeDeleted AuditEventType = "CUSTOM_ENTITY_TYPE_DELETED"
	AuditEventFieldCreated      AuditEventType = "CUSTOM_FIELD_CREATED"
	AuditEventFieldUpdated      AuditEventType = "CUSTOM_FIELD_UPDATED"
	AuditEventFieldDeleted      AuditEventType = "CUSTOM_FIELD_DELETED"
	AuditEventOptionCreated     AuditEventType = "CUSTOM_FIELD_OPTION_CREATED"
	AuditEventOptionUpdated     AuditEventType = "CUSTOM_FIELD_OPTION_UPDATED"
	AuditEventOptionDeleted     AuditEventType = "CUSTOM_FIELD_OPTION_DELETED"
	AuditEventEntityCreated     AuditEventType = "CUSTOM_ENTITY_CREATED"
	AuditEventEntityUpdated     AuditEventType = "CUSTOM_ENTITY_UPDATED"
	AuditEventEntityDeleted     AuditEventType = "CUSTOM_ENTITY_DELETED"
	AuditEventValuesReplaced    AuditEventType = "CUSTOM_FIELD_VALUES_REPLACED"
)

type AuditEvent struct {
	ID           Identifier
	ActorUserID  auth.Identifier
	ResourceKind AuditResourceKind
	ResourceID   *Identifier
	Target       *TargetReference
	EventType    AuditEventType
	Outcome      auth.AuditOutcome
	RequestID    string
}

func normalizeRequestID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "request-id-unavailable"
	}
	if len(value) > 128 {
		return value[:128]
	}
	return value
}
