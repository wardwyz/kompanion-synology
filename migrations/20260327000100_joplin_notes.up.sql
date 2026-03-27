CREATE TABLE IF NOT EXISTS joplin_note (
  id text PRIMARY KEY,
  title text NOT NULL,
  body_md text NOT NULL,
  document_id text,
  source text,
  source_url text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_joplin_note_document_id ON joplin_note(document_id);
CREATE INDEX IF NOT EXISTS idx_joplin_note_created_at ON joplin_note(created_at DESC);
