-- Align persisted OCR evidence with Gymkhana-Core OCR: confidence 0..10000
-- and source-relative regions in millionths [0, 1_000_000].

ALTER TABLE ocr_suggestions
  DROP CONSTRAINT IF EXISTS ocr_suggestions_confidence_check;
ALTER TABLE ocr_suggestions
  ADD CONSTRAINT ocr_suggestions_confidence_check
  CHECK (confidence IS NULL OR confidence BETWEEN 0 AND 10000);

DO $$
DECLARE
  constraint_row record;
BEGIN
  FOR constraint_row IN
    SELECT conname
    FROM pg_constraint
    WHERE conrelid = 'ocr_suggestions'::regclass
      AND contype = 'c'
      AND pg_get_constraintdef(oid) ILIKE '%region_x%'
  LOOP
    EXECUTE format('ALTER TABLE ocr_suggestions DROP CONSTRAINT %I', constraint_row.conname);
  END LOOP;
END $$;

ALTER TABLE ocr_suggestions
  ADD CONSTRAINT ocr_suggestions_region_millionths_check
  CHECK (
    (region_x IS NULL AND region_y IS NULL AND region_width IS NULL AND region_height IS NULL) OR
    (region_x BETWEEN 0 AND 1000000 AND region_y BETWEEN 0 AND 1000000 AND
      region_width BETWEEN 1 AND 1000000 AND region_height BETWEEN 1 AND 1000000 AND
      region_x + region_width <= 1000000 AND region_y + region_height <= 1000000)
  );

INSERT INTO app_metadata (key, value)
VALUES ('schema.ocr_core_evidence_scale', 'v1')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

---- create above / drop below ----

ALTER TABLE ocr_suggestions DROP CONSTRAINT IF EXISTS ocr_suggestions_region_millionths_check;
ALTER TABLE ocr_suggestions DROP CONSTRAINT IF EXISTS ocr_suggestions_confidence_check;

ALTER TABLE ocr_suggestions
  ADD CONSTRAINT ocr_suggestions_confidence_check
  CHECK (confidence IS NULL OR confidence BETWEEN 0 AND 1000);

ALTER TABLE ocr_suggestions
  ADD CONSTRAINT ocr_suggestions_region_check
  CHECK (
    (region_x IS NULL AND region_y IS NULL AND region_width IS NULL AND region_height IS NULL) OR
    (region_x BETWEEN 0 AND 9999 AND region_y BETWEEN 0 AND 9999 AND
      region_width BETWEEN 1 AND 10000 AND region_height BETWEEN 1 AND 10000 AND
      region_x + region_width <= 10000 AND region_y + region_height <= 10000)
  );

DELETE FROM app_metadata WHERE key = 'schema.ocr_core_evidence_scale';
