-- A shaped assistant result is recomputed from the stored plan. It does not
-- need a query execution row.

DO $$
DECLARE
  constraint_name text;
BEGIN
  SELECT conname INTO constraint_name
  FROM pg_constraint
  WHERE conrelid = 'ai_chat_result_references'::regclass
    AND contype = 'c'
    AND pg_get_constraintdef(oid) LIKE '%reference_kind = ''QUERY''%';
  IF constraint_name IS NULL THEN
    RAISE EXCEPTION 'query result reference constraint was not found';
  END IF;
  EXECUTE format('ALTER TABLE ai_chat_result_references DROP CONSTRAINT %I', constraint_name);
END $$;

ALTER TABLE ai_chat_result_references
  ADD CONSTRAINT ai_chat_result_references_kind_execution_check CHECK (
    reference_kind = 'QUERY' OR
    (reference_kind = 'SEARCH' AND query_execution_id IS NULL)
  );

---- create above / drop below ----

ALTER TABLE ai_chat_result_references
  DROP CONSTRAINT ai_chat_result_references_kind_execution_check;

ALTER TABLE ai_chat_result_references
  ADD CONSTRAINT ai_chat_result_references_check CHECK (
    (reference_kind = 'QUERY' AND query_execution_id IS NOT NULL) OR
    (reference_kind = 'SEARCH' AND query_execution_id IS NULL)
  );
