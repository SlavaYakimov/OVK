CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE guests (
    id         BIGSERIAL PRIMARY KEY,
    full_name  TEXT NOT NULL,
    phone      TEXT NOT NULL,
    room       TEXT NOT NULL,
    check_in   DATE NOT NULL,
    check_out  DATE NOT NULL,
    note       TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT dates_ok CHECK (check_out > check_in)
);

-- Поиск: ILIKE '%строка%' по ФИО/телефону/номеру. Обычный B-tree для
-- шаблона с ведущим % бесполезен, GIN + pg_trgm позволяет искать по подстроке.
CREATE INDEX guests_search_trgm ON guests
    USING GIN ((full_name || ' ' || phone || ' ' || room) gin_trgm_ops);
