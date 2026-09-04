# Roles

Roles group [permissions](/docs/workspace/permissions/) together and are assigned to [users](/docs/workspace/users/), either directly or through [groups](/docs/workspace/groups/). Each workspace defines its own roles.

![The workspace's roles: three of them, each with the users holding it, the number of groups carrying it, and how many permission rules it is made of.](/shots/team.roles.light.png)

## Creating roles

Create a role from the team management interface. Give it a name (must be unique within the workspace) and assign permissions.

You can also **duplicate** an existing role to use it as a starting point, then adjust permissions as needed.

## Assigning users

A role can have multiple users, and a user can have multiple roles. Assign a role to a user directly, or attach it to a [group](/docs/workspace/groups/) so every member inherits it. Permissions from all of a user's roles, direct and group-derived, are combined.

## Examples

Common patterns:

- **Read-only**: a role with `SELECT` on all databases, no write permissions
- **Developer**: `SELECT`, `INSERT`, `UPDATE`, `DELETE` on development databases
- **DBA**: full database permissions including `DDL` (schema changes)
- **Admin**: workspace-level permissions for managing users and roles

> [!NOTE]
> Roles have no hierarchy or inheritance. Each role is a flat set of permissions.
