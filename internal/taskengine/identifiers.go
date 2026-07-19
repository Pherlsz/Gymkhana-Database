package taskengine

import (
	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/queryengine"
	"github.com/jackc/pgx/v5/pgtype"
)

func IdentifierFromString(value string) (Identifier, error) {
	return queryengine.ParseIdentifier(value)
}

func optionalTaskAuthUUID(value *auth.Identifier) pgtype.UUID {
	if value == nil || *value == (auth.Identifier{}) {
		return pgtype.UUID{}
	}
	return taskAuthUUID(*value)
}
