package googleforms

import (
	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5/pgtype"
)

func scanActor(row rowScanner) (auth.Session, error) {
	var session auth.Session
	var id pgtype.UUID
	var subject, avatar pgtype.Text
	if err := row.Scan(&id, &subject, &session.User.Email,
		&session.User.DisplayName, &avatar, &session.User.Role, &session.User.Active); err != nil {
		return auth.Session{}, err
	}
	session.User.ID = authIdentifierFromUUID(id)
	if subject.Valid {
		session.User.GoogleSubject = subject.String
	}
	if avatar.Valid {
		session.User.AvatarURL = avatar.String
	}
	return session, nil
}
