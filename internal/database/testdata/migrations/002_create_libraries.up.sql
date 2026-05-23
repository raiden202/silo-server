CREATE TABLE libraries (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT NOT NULL,
    owner_id   BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
