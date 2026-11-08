CREATE TABLE urls (
    short_code VARCHAR(8) NOT NULL PRIMARY KEY,
    original_url TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);