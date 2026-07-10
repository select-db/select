# Groups

Groups collect [users](/workspace/users/) together and grant them [roles](/workspace/roles/). Add someone to a group once, and they inherit every role attached to it, instead of assigning roles to each user one by one. Each workspace defines its own groups.

## Groups vs roles

A [role](/workspace/roles/) is a bundle of [permissions](/workspace/permissions/): *what* you can do. A group is a collection of users (*who* you are) that a set of roles is attached to. A user's effective permissions come from the roles assigned to them directly **plus** the roles from every group they belong to.

## Creating groups

Create a group from the team management interface and give it a name. A new group grants nothing until you attach roles to it.

## Adding members

A group can have many members, and a user can belong to many groups. Adding or removing a member takes effect on that member's next request.

## Attaching roles

Attach one or more [roles](/workspace/roles/) to a group; every member is granted those roles. Detaching a role removes it from all members, unless they also hold it directly or through another group.

## Permissions

Managing groups requires the **Workspace groups** permission (or workspace ownership). Attaching or detaching a role additionally requires the **Workspace roles** permission: granting a role through a group is still granting a role, so it needs the same authority as assigning one directly to a user.

> Groups have no hierarchy: a group cannot contain another group. Members inherit only the roles attached directly to the group.
