-- Stubs for sqlc type inference only. These functions are provided at runtime
-- by the pgcrypto extension (migration 20250710190451_pg_crypto.sql).
CREATE FUNCTION pgp_sym_encrypt(data text, key text) RETURNS bytea AS $$ SELECT NULL::bytea $$ LANGUAGE sql;
CREATE FUNCTION pgp_sym_decrypt(data bytea, key text) RETURNS text AS $$ SELECT NULL::text $$ LANGUAGE sql;
