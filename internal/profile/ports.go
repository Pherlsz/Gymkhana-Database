package profile

import (
	"context"
)

type Store interface {
	Create(context.Context, Identifier, Values) (Profile, error)
	Get(context.Context, Identifier) (Profile, error)
	Count(context.Context, Filters) (int64, error)
	List(context.Context, ListOptions) ([]Profile, error)
	ListByExactFullName(context.Context, string) ([]Profile, error)
	DistinctCities(context.Context, Filters, int32) ([]string, error)
	Update(context.Context, Identifier, int64, Values) (Profile, error)
	Duplicate(context.Context, Identifier, Identifier) (Profile, error)
	Delete(context.Context, Identifier, int64) error
}

type AuditStore interface {
	RecordAuditEvent(context.Context, AuditEvent) error
}
