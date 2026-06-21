CREATE SCHEMA IF NOT EXISTS "app";

CREATE SCHEMA IF NOT EXISTS "audit";

CREATE SCHEMA IF NOT EXISTS "auth";

CREATE SCHEMA IF NOT EXISTS "public";

CREATE SCHEMA IF NOT EXISTS "selectdb";

CREATE SCHEMA IF NOT EXISTS "zygon";

CREATE TABLE app.datasource (
  id uuid,
  workspace_id uuid,
  db_type text,
  encrypted_dsn bytea,
  encrypted_ssh bytea,
  max_open_conns integer,
  max_idle_conns integer,
  conn_max_lifetime integer,
  conn_max_idle_time integer,
  created_at timestamp with time zone,
  updated_at timestamp with time zone,
  name text
);

CREATE TABLE app.permission (
  id uuid,
  role_id uuid,
  workspace_id uuid,
  db_instance_id text,
  schema_name text,
  table_name text,
  column_name text,
  action text,
  effect text,
  updated_at timestamp with time zone,
  deleted_at timestamp with time zone
);

CREATE TABLE app.role (
  id uuid,
  workspace_id uuid,
  name text,
  updated_at timestamp with time zone,
  deleted_at timestamp with time zone
);

CREATE TABLE app."user" (
  id uuid,
  created_at timestamp without time zone,
  github_id bigint,
  name text,
  email text,
  avatar_url text
);

CREATE TABLE app.user_identity (
  id uuid,
  user_id uuid,
  provider text,
  provider_user_id text,
  email text,
  created_at timestamp with time zone
);

CREATE TABLE app.user_to_role (
  id uuid,
  user_id uuid,
  role_id uuid,
  workspace_id uuid,
  updated_at timestamp with time zone,
  deleted_at timestamp with time zone
);

CREATE TABLE app.workspace (
  id uuid,
  name text,
  git_remote_url text,
  updated_at timestamp with time zone,
  deleted_at timestamp with time zone,
  owner_id uuid
);

CREATE TABLE app.workspace_to_user (
  id uuid,
  workspace_id uuid,
  user_id uuid,
  updated_at timestamp with time zone,
  deleted_at timestamp with time zone
);

CREATE TABLE auth.api_key (
  id uuid,
  workspace_id uuid,
  name text,
  prefix text,
  hashed_key text,
  created_by uuid,
  expires_at timestamp with time zone,
  last_used_at timestamp with time zone,
  created_at timestamp with time zone,
  deleted_at timestamp with time zone
);

CREATE TABLE auth.api_key_to_role (
  api_key_id uuid,
  role_id uuid
);

CREATE TABLE auth.refresh_token (
  hashed_token text,
  user_id uuid,
  expires_at timestamp with time zone,
  created_at timestamp with time zone,
  issued_ip inet
);

CREATE TABLE public.goose_db_version (
  id integer,
  version_id bigint,
  is_applied boolean,
  tstamp timestamp without time zone
);

CREATE TABLE audit.principal_snapshot (
  snapshot_hash bytea,
  workspace_id uuid,
  snapshot jsonb,
  created_at timestamp with time zone
);

CREATE TABLE audit.event (
  id uuid,
  workspace_id uuid,
  occurred_at timestamp with time zone,
  recorded_at timestamp with time zone,
  domain text,
  action text,
  principal_hash bytea,
  principal_id uuid,
  principal_type text,
  target_type text,
  target_id uuid,
  target_label text,
  status text,
  payload jsonb,
  duration_ms bigint,
  returned_row_count bigint,
  client_ip inet
);

CREATE TABLE audit.outbox (
  id bigint,
  event_json jsonb,
  enqueued_at timestamp with time zone
);
