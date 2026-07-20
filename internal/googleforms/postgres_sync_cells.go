package googleforms

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Pherlsz/Gymkhana-Database/internal/operations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func loadQuestionsTx(ctx context.Context, tx pgx.Tx, sourceID Identifier) ([]Question, error) {
	rows, err := tx.Query(ctx, `SELECT question_id, position, title, answer_kind,
       required, supported, unsupported_code, question_fingerprint, target_field
  FROM google_forms_questions WHERE source_id=$1 ORDER BY position`, databaseUUID(sourceID))
	if err != nil {
		return nil, fmt.Errorf("load response questions: %w", err)
	}
	defer rows.Close()
	result := make([]Question, 0)
	for rows.Next() {
		var value Question
		var unsupported, target pgtype.Text
		var fingerprint []byte
		if err := rows.Scan(&value.ID, &value.Position, &value.Title, &value.AnswerKind,
			&value.Required, &value.Supported, &unsupported, &fingerprint, &target); err != nil {
			return nil, fmt.Errorf("scan response question: %w", err)
		}
		copy(value.QuestionFingerprint[:], fingerprint)
		if unsupported.Valid {
			value.UnsupportedCode = unsupported.String
		}
		if target.Valid {
			value.TargetField = target.String
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func responseCell(response Response, question Question) (string, operations.ValueKind, string) {
	answers := response.Answers[question.ID]
	if len(answers) > 1 {
		return strings.Join(answers, "; "), operations.ValueText, "google_forms_multiple_answers"
	}
	if len(answers) == 0 || strings.TrimSpace(answers[0]) == "" {
		if question.Required {
			return "", operations.ValueEmpty, "google_forms_required_answer_missing"
		}
		return "", operations.ValueEmpty, ""
	}
	raw := answers[0]
	switch question.AnswerKind {
	case AnswerDate:
		return raw, operations.ValueDate, ""
	case AnswerScale:
		return raw, operations.ValueNumber, ""
	default:
		return raw, operations.ValueText, ""
	}
}

func truncateText(value string, maximum int) string {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	runes := []rune(value)
	return string(runes[:maximum])
}

func integerPointer(value int) *int { return &value }
