DROP INDEX IF EXISTS idx_snippets_fts;
DROP INDEX IF EXISTS idx_snippets_name_trgm;
DROP INDEX IF EXISTS idx_snippets_language;
DROP INDEX IF EXISTS idx_snippets_visibility;
DROP INDEX IF EXISTS idx_snippets_creator_updated;

DROP TRIGGER IF EXISTS trg_snippets_updated_at ON snippets;
DROP FUNCTION IF EXISTS set_updated_at();

DROP TABLE IF EXISTS snippets;
DROP TABLE IF EXISTS users;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_type WHERE typname = 'snippet_visibility') THEN
    DROP TYPE snippet_visibility;
  END IF;
END$$;

DROP EXTENSION IF EXISTS pg_trgm;    