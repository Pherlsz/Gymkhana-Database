package bill

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
	Create(context.Context, Identifier, Values) (Bill, error)
	Get(context.Context, Identifier) (Bill, error)
	Count(context.Context, Filters) (int64, error)
	List(context.Context, ListOptions) ([]Bill, error)
	Update(context.Context, Identifier, int64, Values) (Bill, error)
	Duplicate(context.Context, Identifier, Identifier) (Bill, error)
	Delete(context.Context, Identifier, int64) error
	AssignCurrentUse(context.Context, Identifier, profile.Identifier) (CurrentUse, error)
	ReturnCurrentUse(context.Context, Identifier) error
	GetCurrentUse(context.Context, Identifier) (*CurrentUse, error)
}

type AuditStore interface {
	RecordAuditEvent(context.Context, AuditEvent) error
}

type OwnerResolver interface {
	ListByExactFullName(context.Context, string) ([]profile.Profile, error)
	Create(context.Context, profile.Identifier, profile.Values) (profile.Profile, error)
}
