-- 000001 — extensions, UUIDv7 shim, shared triggers (data-model §1.1/§1.2/§1.3a)
CREATE EXTENSION IF NOT EXISTS pgcrypto;   -- gen_random_uuid()
CREATE EXTENSION IF NOT EXISTS vector;     -- pgvector substrate (ADR-0007/0021)

-- UUIDv7 shim for Postgres 16 (PG18 ships uuidv7() natively). One canonical
-- generator; every insert path resolves DEFAULT uuidv7() to this on PG16.
CREATE OR REPLACE FUNCTION uuidv7() RETURNS uuid AS $$
  SELECT encode(
    set_bit(
      set_bit(
        overlay(uuid_send(gen_random_uuid())
          placing substring(int8send(floor(extract(epoch FROM clock_timestamp()) * 1000)::bigint) FROM 3)
          FROM 1 FOR 6),
        52, 1),
      53, 1),
    'hex')::uuid;
$$ LANGUAGE sql VOLATILE;

-- Bump updated_at on UPDATE (tables without a version column).
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN NEW.updated_at = now(); RETURN NEW; END;
$$ LANGUAGE plpgsql;

-- Bump updated_at + version on UPDATE (mutable domain tables, §1.3a optimistic concurrency).
CREATE OR REPLACE FUNCTION touch_versioned() RETURNS trigger AS $$
BEGIN NEW.updated_at = now(); NEW.version = OLD.version + 1; RETURN NEW; END;
$$ LANGUAGE plpgsql;
