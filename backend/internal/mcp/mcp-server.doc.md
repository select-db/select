# MCP Server

The MCP server lets external LLM clients (Claude Code in VS Code, Cursor, Claude Desktop, …) chat with your [proxified databases](/databases/proxified-connections/). The client connects with an [API key](/workspace/api-keys/), and every database call passes through the same [permission model](/workspace/permissions/) as the app UI.

## What you can do with it

Once connected, the LLM can:

- See which datasources in the workspace it's allowed to query, and what its API key's role permits on each
- List schemas, tables, and views on a chosen datasource
- Read a table's definition (the raw DDL)
- Run read-only queries
- Run writes and DDL, but only if the API key's roles permit it
- Plan or explain a query before running it

Seven tools, no shell access, no file access. The model is bounded to the datasources and actions your roles already let it touch.

## Setup

1. **Create an API key** for the workspace and database access you want this client to have. See [API keys](/workspace/api-keys/). Pick the roles carefully: the MCP server has no extra permission layer of its own, so the key's roles are exactly what the LLM can do.
2. **Add the server to your client.** For Claude Code in VS Code, run from a terminal:
   ```
   claude mcp add --transport http selectdb https://your-backend/mcp \
     --header "Authorization: Bearer sdb_xxxxxxxxxxxxxxxx"
   ```
   Or edit `~/.claude.json` directly:
   ```json
   {
     "mcpServers": {
       "selectdb": {
         "type": "http",
         "url": "https://your-backend/mcp",
         "headers": { "Authorization": "Bearer sdb_xxxxxxxxxxxxxxxx" }
       }
     }
   }
   ```
   The same shape works for VS Code's native MCP support (`.vscode/mcp.json`), Cursor, and Claude Desktop. Project-scoped config in `.mcp.json` at the repo root keeps the key per-project.
3. **Reload the client.** Run `claude mcp list` to verify `selectdb` is connected.

## What gets exposed

| Tool                          | Effect                                                                |
|-------------------------------|-----------------------------------------------------------------------|
| `list_datasources`            | Lists the workspace's datasources the API key's role can touch, each with its permission rules as `effect.action.schema.table.column` strings (e.g. `allow.select.public.*.*`, `deny.select.public.users.password`). Datasources the role has no allow rules on are omitted. |
| `get_database_schemas`        | Lists schemas, tables, and views for a datasource.                    |
| `get_database_table_detail`   | Returns the DDL string for one table or view.                         |
| `execute_query`               | Runs a read-only statement and returns up to 250 rows.                 |
| `execute_statement`           | Runs a write or DDL statement. Host prompts the user via the `destructiveHint` annotation. |
| `plan_query`                  | Returns the planner's tree without executing the query.               |
| `explain_query`               | Executes the query and returns its actual plan (EXPLAIN ANALYZE).     |

## Scope

- **One API key, one workspace.** An API key is bound to a single workspace when it's created. To connect to a different workspace, create a new key there and add a second MCP entry (`selectdb-prod`, `selectdb-staging`, …).
- **Roles decide everything.** A read-only key gets a read-only MCP session because its role lacks `workspace/datasource.execute`. There is no MCP-specific "read-only mode" toggle.
- **Default-deny on unscoped databases.** Unlike the web UI, which silently allows queries on databases the role has no rules for, MCP refuses them. If the key's role has no explicit allow rules for a database, the LLM cannot query that database, even if other roles in the workspace grant access to it. Add an explicit role entry to grant access.
- **API keys only.** The MCP endpoint refuses user JWTs. If you're a logged-in user wanting to query a database, use the web UI; the MCP path is for headless clients with a pre-shared key.

## Security notes

- The API key travels in the `Authorization` header on every request. Use `https://` in production. A leaked key has whatever permissions its roles grant, so rotate or revoke it the same way you would any other credential.
- The LLM sees DDL, table contents up to the row cap, and query plans. Treat the API key's scope as the privacy boundary: anything the roles can read, the LLM (and the LLM provider) can see.
- `execute_statement` carries the MCP `destructiveHint` annotation, so clients like Claude Code and Cursor will prompt you to approve each write before it runs. The role on the API key is what ultimately decides whether the write succeeds; the prompt is the human-in-the-loop on top of that.

> Pair each MCP client with a dedicated, narrowly-scoped API key. If a client only needs to read one database, give its key a role that only grants read on that database, nothing more.
