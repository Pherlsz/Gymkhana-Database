-- Drop unused matching analyses and full-table XLSX export storage.
-- Profile trigram/phone indexes from 015 stay; Search still uses them.

DROP TABLE IF EXISTS matching_audit_events;
DROP TABLE IF EXISTS matching_merge_receipt_counts;
DROP TABLE IF EXISTS matching_merge_receipts;
DROP TABLE IF EXISTS matching_case_decisions;
DROP TABLE IF EXISTS matching_analysis_cases;
DROP TABLE IF EXISTS matching_case_evidence;
DROP TABLE IF EXISTS matching_cases;
DROP TABLE IF EXISTS matching_analyses;
DROP TABLE IF EXISTS matching_rate_limits;

ALTER TABLE operation_audit_events DROP CONSTRAINT IF EXISTS operation_audit_events_export_id_fkey;
ALTER TABLE operation_audit_events DROP COLUMN IF EXISTS export_id;

DROP TABLE IF EXISTS operation_exports;

DELETE FROM app_user_capabilities WHERE capability = 'MATCHING';

INSERT INTO app_metadata (key, value)
VALUES ('schema.unused_p2_retired', 'matching-export-v1')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

DELETE FROM app_metadata WHERE key = 'schema.unused_p2_retired';

-- Recreate export/matching tables only by rolling back to 015/012 if needed.
-- This down section is intentionally empty of table rebuilds.
