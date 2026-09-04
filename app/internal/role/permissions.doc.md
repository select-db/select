# Permissions

Permissions control what users can do in a workspace. They are assigned to [roles](/docs/workspace/roles/) at varying levels of granularity.

![The permission grid for a role: workspace-level rows above, then the database, its schema and its tables, with allow and deny against each action.](/shots/permissions.light.png)

## Permission levels

Permissions are hierarchical. You can grant or deny access at five levels, from broad to specific:

| Level       | Scope                                           |
|-------------|--------------------------------------------------|
| **App**     | Workspace settings, user, role, and group management |
| **Database**| All schemas, tables, and columns in a database   |
| **Schema**  | All tables and columns in a schema               |
| **Table**   | All columns in a table                           |
| **Column**  | A specific column                                |

A permission set at the database level applies to everything inside it unless overridden at a more specific level.

## Actions

Permissions are defined per action:

| Action     | Description                              |
|------------|------------------------------------------|
| **SELECT** | Read data from tables                    |
| **INSERT** | Add new rows                             |
| **UPDATE** | Modify existing rows                     |
| **DELETE** | Remove rows                              |
| **DDL**    | Schema changes (CREATE, ALTER, DROP)     |

App-level actions cover workspace administration:

| Action                    | Description                                      |
|---------------------------|--------------------------------------------------|
| **Workspace settings**    | Edit workspace name, git remote, and settings    |
| **Workspace users**       | Invite, remove, and manage members               |
| **Workspace roles**       | Create, edit, and delete roles and permissions   |
| **Workspace groups**      | Create groups and manage members; attaching a role to a group also requires Workspace roles |
| **Workspace API keys**    | Create, rotate, and revoke API keys              |

API keys let automated clients authenticate with the roles bound to the key, so every query they run passes through this same permission model.

## Allow and deny

Each permission has an **effect**: `allow` or `deny`. When a user has multiple roles, all permissions are combined. **Deny always wins**: if one role allows an action and another denies it, the action is denied.

Evaluation order:

1. SELECT checks all deny rules first across all of the user's roles
2. If any deny matches, access is refused (regardless of allow rules)
3. If no deny matches, SELECT checks allow rules
4. If an allow matches, access is granted
5. If neither matches, access is refused (default deny)

This lets you create broad access with targeted restrictions. For example, a "Developer" role can allow all operations, while an "Intern" role adds a deny on `DDL` for production databases. A user with both roles cannot run DDL on production.

## Databases nobody has written a rule for

Step 5 is about a database your roles *do* cover. A database that no role in the
workspace mentions at all is a different case, and the answer depends on who is
holding the credentials.

| Connection | A database with no rules | Why |
|------------|--------------------------|-----|
| **Local** | Allowed | The DSN is yours, on your machine. Refusing here would only be refusing you access to your own database. |
| **[Proxified](/docs/databases/proxified-connections/)** | Refused | The credentials are held server-side and the query runs on our server. Nothing is granted that a role did not grant. |
| **[MCP](/docs/workspace/mcp-server/)** | Refused | Same server, same rule. An agent gets what its key's roles name, and nothing else. |

> [!IMPORTANT]
> Adding a database to a proxified workspace does not make it readable. Until a
> role carries an allow rule for it, every query against it is refused,
> including from an API key that can read every other database in the
> workspace.

## Scope

When adding a permission, you choose its scope using checkboxes. Granting access at the database level automatically covers all schemas, tables, and columns inside it. You can then narrow it down by selecting a specific schema, table, or column.

## Example setup

A typical team might have:

| Role        | Database    | Action               | Effect |
|-------------|-------------|----------------------|--------|
| Developer   | dev-db      | SELECT, INSERT, UPDATE, DELETE | allow |
| Developer   | prod-db     | SELECT               | allow  |
| DBA         | *           | SELECT, INSERT, UPDATE, DELETE, DDL | allow |
| Analyst     | prod-db     | SELECT               | allow  |
