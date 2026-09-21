# Permissions

Permissions control what users can do in a workspace. They are assigned to [roles](/docs/workspace/roles/) at varying levels of granularity.

![The permission grid for a role: workspace-level rows above, then the database, its schema and its tables, with allow and deny against each action.](/shots/permissions.light.webp)

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
| **MANAGE** | Change the database itself: its structure, its access, its configuration |
| **SEE**    | Read the values in a column. Without it the column's cells come back masked |

App-level actions cover workspace administration:

| Action                    | Description                                      |
|---------------------------|--------------------------------------------------|
| **Workspace settings**    | Edit workspace name, git remote, and settings    |
| **Workspace users**       | Invite, remove, and manage members               |
| **Workspace roles**       | Create, edit, and delete roles and permissions   |
| **Workspace groups**      | Create groups and manage members; attaching a role to a group also requires Workspace roles |
| **Workspace API keys**    | Create, rotate, and revoke API keys              |

API keys let automated clients authenticate with the roles bound to the key, so every query they run passes through this same permission model.

### Hiding a column

SELECT says which columns a role may query; SEE says which values it may read.
A role holding SELECT but not SEE on a column gets `*****` where its cells would
be, and may not use the column any other way.

A statement that tests a hidden column is refused rather than run. Answering
"are there rows where email starts with a" is answering a question about the
values, and enough answers are the value; the row count of an update filtered
the same way answers it just as well, which is why the refusal does not wait for
a statement to return rows. Hiding a column from the eye is not hiding it, so
SEE hides it from the query. A subquery that selects the column to compare it
against something is the same test written longer, and is refused too: write
`EXISTS (SELECT 1 FROM ...)` where the column was only there to fill the list.

WHERE is not the only clause that asks. GROUP BY, ORDER BY, HAVING, a join
condition and a window clause each read a column without returning it, and each
answers a question a row at a time: which group a row falls in, which of two
rows sorts first, whether a join matched. They are refused on the same terms as
a WHERE, whether the join names its columns in an ON, in a USING list or in
neither, as NATURAL does, and whether the statement reading them is a select or
the FROM of an UPDATE. `FILTER (WHERE ...)` and the ORDER BY or PARTITION BY of
an `OVER (...)` ask the same way. SELECT DISTINCT is refused too, since
collapsing duplicate rows makes the row count the number of distinct values in
the columns it names, and so is the predicate of an ON CONFLICT, which chooses
which stored rows an upsert changes.

The whole statement is read, so a column hidden at the bottom stays hidden when
a derived table or a CTE hands it up, and the rows a write returns through
RETURNING are masked like any others. Renaming it on the way up does not unhide
it: a statement testing `s.email` where `s` is a derived table is refused like
one testing `users.email`.

A write is refused where it reads a hidden column, rather than masked. Masking
works on the rows a statement hands back, and what a write reads it stores:
`INSERT INTO other (c) SELECT email FROM users` would leave the caller a table
it may select from, holding the values SEE refused it. Writing to a hidden
column is not reading it, and stays allowed.

Where a result column cannot be traced back to a column of a table, and a hidden
column in the statement has no result column of its own, the statement is
refused rather than run: an expression over a hidden column, a subquery
returning it under a name of its own, and a column added since the metadata was
last read all land here. A literal, a count or a window function beside a hidden
column is not one of these, since the hidden column still has its own position
to mask.

Where two scopes spell a column the same way, the statement's own result columns
say which one comes out.

## Allow and deny

Each permission has an **effect**: `allow` or `deny`. When a user has multiple roles, all permissions are combined. **Deny always wins**: if one role allows an action and another denies it, the action is denied.

Evaluation order:

1. SELECT checks all deny rules first across all of the user's roles
2. If any deny matches, access is refused (regardless of allow rules)
3. If no deny matches, SELECT checks allow rules
4. If an allow matches, access is granted
5. If neither matches, access is refused (default deny)

This lets you create broad access with targeted restrictions. For example, a "Developer" role can allow all operations, while an "Intern" role adds a deny on `MANAGE` for production databases. A user with both roles cannot change the schema on production.

The first four actions are about the rows in a table. MANAGE is about the
database itself: creating, altering and dropping tables, granting access,
loading and exporting in bulk, and running procedures. It is granted on a
connection, not on a schema or a table, so a rule scoped below the connection
never matches.

Anything that is not plainly one of the four row actions needs MANAGE. That is
deliberate: granting the four never quietly grants more than reading and
writing rows, so an unusual statement is refused rather than let through.

A statement that does two things needs the permission for both. `CREATE TABLE
new AS SELECT * FROM old` builds a table and reads another one, so it needs
MANAGE and SELECT on `old`. The same holds for a view over a table, an UPDATE
that reads a second table to fill the first, and a statement whose result is
built by a query inside it.

Every table a statement names counts, including one no column is selected from.
`SELECT t1.c2 FROM t1, t2` reads `t2` for its rows whether or not a column of it
appears in the result, and a `WHERE` over it reports what those rows hold, so it
needs SELECT on `t2` as well as on `t1`.

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
| DBA         | *           | SELECT, INSERT, UPDATE, DELETE, MANAGE | allow |
| Analyst     | prod-db     | SELECT               | allow  |
