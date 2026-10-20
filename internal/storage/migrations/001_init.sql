CREATE TABLE IF NOT EXISTS documents (
    id              uuid        PRIMARY KEY,
    title           text        NOT NULL,
    category        text        NOT NULL DEFAULT '',
    body            text        NOT NULL DEFAULT '',
    source          text        NOT NULL DEFAULT '',
    created_at      timestamptz NOT NULL,
    updated_at      timestamptz NOT NULL,
    deleted_at      timestamptz,
    current_version bigint      NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS versions (
    id             uuid        PRIMARY KEY,
    document_id    uuid        NOT NULL REFERENCES documents (id) ON DELETE CASCADE,
    number         bigint      NOT NULL,
    body           text        NOT NULL DEFAULT '',
    meta_snapshot  jsonb       NOT NULL DEFAULT '{}'::jsonb,
    created_at     timestamptz NOT NULL,
    UNIQUE (document_id, number)
);

CREATE TABLE IF NOT EXISTS tags (
    id   uuid   PRIMARY KEY,
    name text   NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS document_tags (
    document_id uuid NOT NULL REFERENCES documents (id) ON DELETE CASCADE,
    tag_id      uuid NOT NULL REFERENCES tags (id)      ON DELETE CASCADE,
    PRIMARY KEY (document_id, tag_id)
);

CREATE INDEX IF NOT EXISTS documents_title_idx       ON documents (title);
CREATE INDEX IF NOT EXISTS documents_category_idx    ON documents (category);
CREATE INDEX IF NOT EXISTS documents_updated_at_idx  ON documents (updated_at);
CREATE INDEX IF NOT EXISTS versions_document_idx     ON versions (document_id, number);
CREATE INDEX IF NOT EXISTS document_tags_document_idx ON document_tags (document_id);
CREATE INDEX IF NOT EXISTS document_tags_tag_idx      ON document_tags (tag_id);