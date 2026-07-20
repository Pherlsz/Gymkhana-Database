package aichat

import (
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

type rowScanner interface{ Scan(...any) error }

const threadSelect = `SELECT id,owner_user_id,title,active_result_reference_id,
retention_expires_at,version,created_at,updated_at FROM ai_chat_threads`

const runSelect = `SELECT id,thread_id,owner_user_id,retry_of_run_id,idempotency_key,
request_fingerprint,state,tool_call_count,input_usage,output_usage,result_bytes,error_code,
cancel_requested_at,started_at,completed_at,version,created_at,updated_at FROM ai_chat_runs`
