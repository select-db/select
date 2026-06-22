CREATE SCHEMA IF NOT EXISTS "app";

CREATE SCHEMA IF NOT EXISTS "audit";

CREATE SCHEMA IF NOT EXISTS "auth";

CREATE SCHEMA IF NOT EXISTS "partman";

CREATE SCHEMA IF NOT EXISTS "public";

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

CREATE TABLE audit.event_auth (
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

CREATE TABLE audit.event_auth_default (
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

CREATE TABLE audit.event_auth_p20260201 (
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

CREATE TABLE audit.event_auth_p20260301 (
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

CREATE TABLE audit.event_auth_p20260401 (
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

CREATE TABLE audit.event_auth_p20260501 (
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

CREATE TABLE audit.event_auth_p20260601 (
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

CREATE TABLE audit.event_auth_p20260701 (
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

CREATE TABLE audit.event_auth_p20260801 (
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

CREATE TABLE audit.event_auth_p20260901 (
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

CREATE TABLE audit.event_auth_p20261001 (
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

CREATE TABLE audit.event_datasource (
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

CREATE TABLE audit.event_datasource_default (
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

CREATE TABLE audit.event_datasource_p20260201 (
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

CREATE TABLE audit.event_datasource_p20260301 (
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

CREATE TABLE audit.event_datasource_p20260401 (
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

CREATE TABLE audit.event_datasource_p20260501 (
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

CREATE TABLE audit.event_datasource_p20260601 (
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

CREATE TABLE audit.event_datasource_p20260701 (
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

CREATE TABLE audit.event_datasource_p20260801 (
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

CREATE TABLE audit.event_datasource_p20260901 (
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

CREATE TABLE audit.event_datasource_p20261001 (
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

CREATE TABLE audit.event_iam (
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

CREATE TABLE audit.event_iam_default (
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

CREATE TABLE audit.event_iam_p20260201 (
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

CREATE TABLE audit.event_iam_p20260301 (
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

CREATE TABLE audit.event_iam_p20260401 (
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

CREATE TABLE audit.event_iam_p20260501 (
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

CREATE TABLE audit.event_iam_p20260601 (
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

CREATE TABLE audit.event_iam_p20260701 (
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

CREATE TABLE audit.event_iam_p20260801 (
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

CREATE TABLE audit.event_iam_p20260901 (
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

CREATE TABLE audit.event_iam_p20261001 (
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

CREATE TABLE audit.event_query (
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

CREATE TABLE audit.event_query_default (
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

CREATE TABLE audit.event_query_p20260201 (
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

CREATE TABLE audit.event_query_p20260301 (
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

CREATE TABLE audit.event_query_p20260401 (
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

CREATE TABLE audit.event_query_p20260501 (
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

CREATE TABLE audit.event_query_p20260601 (
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

CREATE TABLE audit.event_query_p20260701 (
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

CREATE TABLE audit.event_query_p20260801 (
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

CREATE TABLE audit.event_query_p20260901 (
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

CREATE TABLE audit.event_query_p20261001 (
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

CREATE TABLE audit.principal_snapshot (
  snapshot_hash bytea,
  workspace_id uuid,
  snapshot jsonb,
  created_at timestamp with time zone
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

CREATE TABLE partman.part_config (
  parent_table text,
  control text,
  time_encoder text,
  time_decoder text,
  partition_interval text,
  partition_type text,
  premake integer,
  automatic_maintenance text,
  template_table text,
  retention text,
  retention_schema text,
  retention_keep_index boolean,
  retention_keep_table boolean,
  epoch text,
  constraint_cols _text,
  optimize_constraint integer,
  infinite_time_partitions boolean,
  datetime_string text,
  jobmon boolean,
  sub_partition_set_full boolean,
  undo_in_progress boolean,
  inherit_privileges boolean,
  constraint_valid boolean,
  ignore_default_data boolean,
  date_trunc_interval text,
  maintenance_order integer,
  retention_keep_publication boolean,
  maintenance_last_run timestamp with time zone
);

CREATE TABLE partman.part_config_sub (
  sub_parent text,
  sub_control text,
  sub_time_encoder text,
  sub_time_decoder text,
  sub_partition_interval text,
  sub_partition_type text,
  sub_premake integer,
  sub_automatic_maintenance text,
  sub_template_table text,
  sub_retention text,
  sub_retention_schema text,
  sub_retention_keep_index boolean,
  sub_retention_keep_table boolean,
  sub_epoch text,
  sub_constraint_cols _text,
  sub_optimize_constraint integer,
  sub_infinite_time_partitions boolean,
  sub_jobmon boolean,
  sub_inherit_privileges boolean,
  sub_constraint_valid boolean,
  sub_ignore_default_data boolean,
  sub_default_table boolean,
  sub_date_trunc_interval text,
  sub_maintenance_order integer,
  sub_retention_keep_publication boolean,
  sub_control_not_null boolean
);

CREATE TABLE partman.template_audit_event_auth (
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

CREATE TABLE partman.template_audit_event_datasource (
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

CREATE TABLE partman.template_audit_event_iam (
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

CREATE TABLE partman.template_audit_event_query (
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

CREATE TABLE public.goose_db_version (
  id integer,
  version_id bigint,
  is_applied boolean,
  tstamp timestamp without time zone
);
