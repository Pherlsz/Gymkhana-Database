package aichat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5/pgtype"
)

func (store *PostgresStore) ListMessages(ctx context.Context, threadID Identifier, owner auth.Identifier, limit, offset int) (MessagePage, error) {
	if _, err := store.GetThread(ctx, threadID, owner); err != nil {
		return MessagePage{}, err
	}
	rows, err := store.pool.Query(ctx, `SELECT message.id,message.thread_id,message.run_id,message.sequence,message.role,message.content,message.created_at,
COALESCE((SELECT jsonb_agg(reference.id ORDER BY reference.created_at,reference.id)
  FROM ai_chat_result_references reference
  WHERE reference.run_id=message.run_id AND message.role='ASSISTANT'),'[]'::jsonb)
FROM ai_chat_messages message WHERE message.thread_id=$1 ORDER BY message.sequence,message.id LIMIT $2 OFFSET $3`, chatUUID(threadID), limit, offset)
	if err != nil {
		return MessagePage{}, fmt.Errorf("list AI Chat messages: %w", err)
	}
	defer rows.Close()
	page := MessagePage{Messages: make([]Message, 0), Limit: limit, Offset: offset}
	for rows.Next() {
		value, err := scanMessageWithReferences(rows)
		if err != nil {
			return MessagePage{}, fmt.Errorf("scan AI Chat message: %w", err)
		}
		page.Messages = append(page.Messages, value)
	}
	if err := rows.Err(); err != nil {
		return MessagePage{}, fmt.Errorf("iterate AI Chat messages: %w", err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM ai_chat_messages WHERE thread_id=$1`, chatUUID(threadID)).Scan(&page.Total); err != nil {
		return MessagePage{}, fmt.Errorf("count AI Chat messages: %w", err)
	}
	return page, nil
}

func scanMessage(row rowScanner) (Message, error) {
	var value Message
	var id, thread, run pgtype.UUID
	if err := row.Scan(&id, &thread, &run, &value.Sequence, &value.Role, &value.Content, &value.CreatedAt); err != nil {
		return Message{}, err
	}
	value.ID, value.ThreadID, value.RunID = chatIdentifier(id), chatIdentifier(thread), chatIdentifier(run)
	return value, nil
}

func scanMessageWithReferences(row rowScanner) (Message, error) {
	var value Message
	var id, thread, run pgtype.UUID
	var encodedReferences []byte
	if err := row.Scan(&id, &thread, &run, &value.Sequence, &value.Role, &value.Content, &value.CreatedAt, &encodedReferences); err != nil {
		return Message{}, err
	}
	value.ID, value.ThreadID, value.RunID = chatIdentifier(id), chatIdentifier(thread), chatIdentifier(run)
	var references []string
	if err := json.Unmarshal(encodedReferences, &references); err != nil || len(references) > MaximumToolCalls {
		return Message{}, ErrInvalidState
	}
	value.ResultReferenceIDs = make([]Identifier, 0, len(references))
	for _, raw := range references {
		reference, err := ParseIdentifier(raw)
		if err != nil {
			return Message{}, ErrInvalidState
		}
		value.ResultReferenceIDs = append(value.ResultReferenceIDs, reference)
	}
	return value, nil
}
