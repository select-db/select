# Roles

Roles group [permissions](/workspace/permissions/) together and are assigned to [users](/workspace/users/). Each workspace defines its own roles.

## Creating roles

Create a role from the team management interface. Give it a name (must be unique within the workspace) and assign permissions.

You can also **duplicate** an existing role to use it as a starting point, then adjust permissions as needed.

## Assigning users

A role can have multiple users, and a user can have multiple roles. Permissions from all of a user's roles are combined.

## Examples

Common patterns:

- **Read-only**: a role with `SELECT` on all databases, no write permissions
- **Developer**: `SELECT`, `INSERT`, `UPDATE`, `DELETE` on development databases
- **DBA**: full database permissions including `DDL` (schema changes)
- **Admin**: workspace-level permissions for managing users and roles

> Roles have no hierarchy or inheritance. Each role is a flat set of permissions.
