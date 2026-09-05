package document

import (
	"context"

	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
)

type Store interface {
	CreateType(context.Context, Identifier, TypeValues) (TypeDefinition, error)
	GetType(context.Context, Identifier) (TypeDefinition, error)
	CountTypes(context.Context, TypeFilters) (int64, error)
	ListTypes(context.Context, TypeListOptions) ([]TypeDefinition, error)
	UpdateType(context.Context, Identifier, int64, TypeValues) (TypeDefinition, error)
	DeleteType(context.Context, Identifier, int64) error
	Create(context.Context, Identifier, Values) (Document, error)
	Get(context.Context, Identifier) (Document, error)
	Count(context.Context, Filters) (int64, error)
	List(context.Context, ListOptions) ([]Document, error)
	Update(context.Context, Identifier, int64, Values) (Document, error)
	Duplicate(context.Context, Identifier, Identifier) (Document, error)
	Delete(context.Context, Identifier, int64) error
	AssignCurrentUse(context.Context, Identifier, profile.Identifier) (CurrentUse, error)
	ReturnCurrentUse(context.Context, Identifier) error
	GetCurrentUse(context.Context, Identifier) (*CurrentUse, error)
	UpsertPresence(context.Context, profile.Identifier, Identifier, Claim, string) (Presence, error)
}

type AuditStore interface {
	RecordAuditEvent(context.Context, AuditEvent) error
}
