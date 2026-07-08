<script lang="ts">
	import Table from '$lib/system/Table/Table.svelte';
	import Avatar from '$lib/system/Avatar/Avatar.svelte';
	import Button from '$lib/system/Button/Button.svelte';
	import { must, tryCatch } from '$lib/utils/tryCatch';
	import {
		ListWorkspaceUsers,
		SearchUser,
		AddUserToWorkspace,
		RemoveUserFromWorkspace
	} from '$lib/wailsjs/go/workspace/Workspace';
	import { ListRoles, AssignUserToRole, RemoveUserFromRole } from '$lib/wailsjs/go/role/Role';
	import { ListGroups, AddUserToGroup, RemoveUserFromGroup } from '$lib/wailsjs/go/group/Group';
	import { GetCurrentUserAvatar } from '$lib/wailsjs/go/user/User';
	import FloatingBox from '$lib/system/FloatingBox/FloatingBox.svelte';
	import Menu from '$lib/system/Menu/Menu.svelte';
	import Portal from '$lib/system/Portal/Portal.svelte';
	import { EventsOn, EventsOff } from '$lib/wailsjs/runtime/runtime';
	import { myPermissions } from '$lib/stores/myPermissionsStore';
	import { notify } from '$lib/system/Notifications/notificationsStore';
	import { AlertType } from '$lib/system/Alert/types';
	import { isEmail } from '$lib/utils/isEmail';
	import type { workspace, generated } from '$lib/wailsjs/go/models';

	type UserEntry = workspace.WorkspaceUserEntry;

	const columns = [
		{ key: 'name', label: 'Name', searchable: true, width: '200px', pinned: true },
		{ key: 'email', label: 'Email', width: '280px' },
		{ key: 'roles', label: 'Roles', width: '220px' },
		{ key: 'groups', label: 'Groups', width: '220px' },
		{ key: 'actions', label: '', width: '52px' }
	];

	let users = $state<UserEntry[]>([]);
	let allRoles = $state<generated.ListRolesByWorkspaceRow[]>([]);
	let allGroups = $state<generated.ListGroupsByWorkspaceRow[]>([]);
	let avatars = $state<Record<string, string>>({});
	let menuAnchor = $state<HTMLElement | null>(null);
	let menuUserId = $state<string | null>(null);
	let roleSearch = $state('');
	let groupMenuAnchor = $state<HTMLElement | null>(null);
	let groupMenuUserId = $state<string | null>(null);
	let groupSearch = $state('');
	let actionsAnchor = $state<HTMLElement | null>(null);
	let actionsUserId = $state<string | null>(null);

	let canManageUsers = $derived($myPermissions.isAllowed('workspace/users.manage'));

	// Add user menu state
	let addUserAnchor = $state<HTMLElement | null>(null);
	let addUserSearch = $state('');
	let addUserLoading = $state(false);
	let addUserOptions = $state<
		{ id: string; label: string; disabled?: boolean; action: () => Promise<void> }[]
	>([]);

	function openAddUserMenu(e: MouseEvent) {
		addUserSearch = '';
		addUserOptions = [];
		addUserAnchor = e.currentTarget as HTMLElement;
	}

	function closeAddUserMenu() {
		addUserAnchor = null;
		addUserSearch = '';
		addUserOptions = [];
	}

	async function addUserByEmail(email: string) {
		const [, err] = await tryCatch(AddUserToWorkspace, email);
		if (err) {
			notify({ type: AlertType.Error, message: (err as Error)?.message ?? 'Failed to add user' });
			return;
		}
		notify({ type: AlertType.Success, message: `Added ${email}` });
		closeAddUserMenu();
		await load();
	}

	let addUserMenuOptions = $derived.by(() => {
		if (addUserOptions.length > 0) return addUserOptions;
		return [];
	});

	let searchDebounceTimer: ReturnType<typeof setTimeout> | null = null;

	$effect(() => {
		const email = addUserSearch.trim().toLowerCase();
		addUserOptions = [];

		if (searchDebounceTimer) clearTimeout(searchDebounceTimer);

		if (!email || !isEmail(email)) return;

		searchDebounceTimer = setTimeout(() => searchUser(email), 300);
	});

	async function searchUser(email: string) {
		addUserLoading = true;
		const [result, err] = await tryCatch(SearchUser, email);
		addUserLoading = false;

		// Query is stale, user kept typing
		if (addUserSearch.trim().toLowerCase() !== email) return;

		if (err) {
			addUserOptions = [
				{
					id: 'error',
					label: (err as Error)?.message ?? 'Search failed',
					disabled: true,
					action: async () => {}
				}
			];
			return;
		}

		const sr = result as workspace.SearchUserResult;

		if (sr.already_added) {
			addUserOptions = [
				{
					id: 'already',
					label: `${sr.name ?? sr.email}, already in workspace`,
					disabled: true,
					action: async () => {}
				}
			];
			return;
		}

		if (sr.found) {
			addUserOptions = [
				{
					id: sr.user_id ?? email,
					label: sr.name ? `Add ${sr.name} (${sr.email})` : `Add ${sr.email}`,
					action: () => addUserByEmail(email)
				}
			];
			return;
		}

		addUserOptions = [
			{
				id: 'invite',
				label: `Invite ${email}`,
				action: () => addUserByEmail(email)
			}
		];
	}

	async function load() {
		const [u, r, g] = await Promise.all([
			must(tryCatch(ListWorkspaceUsers)),
			must(tryCatch(ListRoles)),
			must(tryCatch(ListGroups))
		]);
		users = (u as UserEntry[]) ?? [];
		allRoles = (r as generated.ListRolesByWorkspaceRow[]) ?? [];
		allGroups = (g as generated.ListGroupsByWorkspaceRow[]) ?? [];
		loadAvatars(users);
	}

	async function loadAvatars(userList: UserEntry[]) {
		for (const u of userList) {
			if (avatars[u.id]) continue;
			const [src] = await tryCatch(GetCurrentUserAvatar, u.id);
			if (src) avatars = { ...avatars, [u.id]: src as string };
		}
	}

	$effect(() => {
		load();
	});

	$effect(() => {
		EventsOn('rolesUpdated', load);
		return () => EventsOff('rolesUpdated');
	});

	function openRoleMenu(anchor: HTMLElement, userId: string) {
		roleSearch = '';
		menuAnchor = anchor;
		menuUserId = userId;
	}

	function closeMenu() {
		menuAnchor = null;
		menuUserId = null;
	}

	let menuUser = $derived(users.find((u) => u.id === menuUserId) ?? null);
	let assignedRoleIds = $derived(new Set(menuUser?.roles?.map((r) => r.id) ?? []));

	async function toggleRole(roleId: string) {
		if (!menuUser) return;
		if (assignedRoleIds.has(roleId)) {
			const match = menuUser.roles?.find((r) => r.id === roleId);
			if (!match) return;
			await must(tryCatch(RemoveUserFromRole, match.user_to_role_id));
		} else {
			await must(tryCatch(AssignUserToRole, menuUser.id, roleId));
		}
		await load();
	}

	let roleMenuOptions = $derived(
		allRoles.map((r) => ({
			id: r.id,
			label: r.name,
			checkbox: true,
			checked: assignedRoleIds.has(r.id),
			action: (opt: { id: string }) => toggleRole(opt.id),
			onCheckClick: (opt: { id: string }) => toggleRole(opt.id)
		}))
	);

	let filteredRoleOptions = $derived.by(() => {
		const q = roleSearch.trim().toLowerCase();
		if (!q) return roleMenuOptions;
		return roleMenuOptions.filter((o) => o.label.toLowerCase().includes(q));
	});

	function openGroupMenu(anchor: HTMLElement, userId: string) {
		groupSearch = '';
		groupMenuAnchor = anchor;
		groupMenuUserId = userId;
	}

	function closeGroupMenu() {
		groupMenuAnchor = null;
		groupMenuUserId = null;
	}

	let groupMenuUser = $derived(users.find((u) => u.id === groupMenuUserId) ?? null);
	let assignedGroupIds = $derived(new Set(groupMenuUser?.groups?.map((g) => g.id) ?? []));

	async function toggleGroup(groupId: string) {
		if (!groupMenuUser) return;
		if (assignedGroupIds.has(groupId)) {
			const match = groupMenuUser.groups?.find((g) => g.id === groupId);
			if (!match) return;
			await must(tryCatch(RemoveUserFromGroup, match.user_to_group_id));
		} else {
			await must(tryCatch(AddUserToGroup, groupMenuUser.id, groupId));
		}
		await load();
	}

	let groupMenuOptions = $derived(
		allGroups.map((g) => ({
			id: g.id,
			label: g.name,
			checkbox: true,
			checked: assignedGroupIds.has(g.id),
			action: (opt: { id: string }) => toggleGroup(opt.id),
			onCheckClick: (opt: { id: string }) => toggleGroup(opt.id)
		}))
	);

	let filteredGroupOptions = $derived.by(() => {
		const q = groupSearch.trim().toLowerCase();
		if (!q) return groupMenuOptions;
		return groupMenuOptions.filter((o) => o.label.toLowerCase().includes(q));
	});

	async function removeUser(userId: string) {
		const [, err] = await tryCatch(RemoveUserFromWorkspace, userId);
		if (err) {
			notify({
				type: AlertType.Error,
				message: (err as Error)?.message ?? 'Failed to remove user'
			});
			return;
		}
		await load();
	}
</script>

<div class="users-panel">
	<div class="section">
		<p>Users</p>
		{#if canManageUsers}
			<Button
				content="New user"
				emphasis="high"
				size="sm"
				onclick={openAddUserMenu}
				style="height: 22px; padding: 0 var(--space-xs);"
			/>
		{/if}
	</div>
	<div class="users-table-wrap">
		<Table
			{columns}
			rows={users}
			getKey={(u) => u.id}
			filterValue={(u) => [u.name ?? '', u.email ?? ''].join(' ')}
			{cell}
		/>
	</div>
</div>

{#snippet cell(key: string, user: UserEntry)}
	{#if key === 'name'}
		<div class="cell-user">
			<Avatar name={user.name} src={avatars[user.id]} size={16} />
			<p class="user-name selectable truncate">{user.name ?? user.id}</p>
		</div>
	{:else if key === 'email'}
		<span class="text-muted selectable">{user.email ?? '-'}</span>
	{:else if key === 'roles'}
		<div class="cell-roles">
			{#if user.is_owner}
				<span class="badge-owner">Owner</span>
			{/if}
			{#if user.roles && user.roles.length > 0}
				<Button
					content={user.roles.map((r) => r.name).join(', ')}
					emphasis="low"
					size="sm"
					onclick={(e) => {
						openRoleMenu(e.currentTarget as HTMLElement, user.id);
					}}
					truncate
				/>
			{:else}
				<Button
					content="Add role"
					emphasis="low"
					size="sm"
					onclick={(e) => {
						openRoleMenu(e.currentTarget as HTMLElement, user.id);
					}}
				/>
			{/if}
		</div>
	{:else if key === 'groups'}
		<div class="cell-roles">
			{#if user.groups && user.groups.length > 0}
				<Button
					content={user.groups.map((g) => g.name).join(', ')}
					emphasis="low"
					size="sm"
					onclick={(e) => {
						openGroupMenu(e.currentTarget as HTMLElement, user.id);
					}}
					truncate
				/>
			{:else}
				<Button
					content="Add group"
					emphasis="low"
					size="sm"
					onclick={(e) => {
						openGroupMenu(e.currentTarget as HTMLElement, user.id);
					}}
				/>
			{/if}
		</div>
	{:else if key === 'actions'}
		{#if !user.is_owner && canManageUsers}
			<div class="actions-inner">
				<span class="action-btn-wrap">
					<Button
						leftIcon="dots"
						iconSize={14}
						emphasis="low"
						size="sm"
						onclick={(e) => {
							e.stopPropagation();
							actionsAnchor = e.currentTarget as HTMLElement;
							actionsUserId = actionsUserId === user.id ? null : user.id;
						}}
					/>
				</span>
			</div>
		{/if}
	{/if}
{/snippet}

{#if menuUserId && menuAnchor}
	<Portal>
		<FloatingBox anchor={menuAnchor} backdrop onBackdropClick={closeMenu}>
			<Menu
				options={filteredRoleOptions}
				searchEnabled={true}
				searchPlaceholder="Search roles…"
				bind:searchQuery={roleSearch}
				externalFilter={true}
				emptyMessage="No roles"
				noResultsMessage="No matching roles"
				width={220}
				onClose={closeMenu}
			/>
		</FloatingBox>
	</Portal>
{/if}

{#if groupMenuUserId && groupMenuAnchor}
	<Portal>
		<FloatingBox anchor={groupMenuAnchor} backdrop onBackdropClick={closeGroupMenu}>
			<Menu
				options={filteredGroupOptions}
				searchEnabled={true}
				searchPlaceholder="Search groups…"
				bind:searchQuery={groupSearch}
				externalFilter={true}
				emptyMessage="No groups"
				noResultsMessage="No matching groups"
				width={220}
				onClose={closeGroupMenu}
			/>
		</FloatingBox>
	</Portal>
{/if}

{#if actionsUserId && actionsAnchor}
	<Portal>
		<FloatingBox
			anchor={actionsAnchor}
			backdrop
			onBackdropClick={() => {
				actionsAnchor = null;
				actionsUserId = null;
			}}
		>
			<Menu
				options={[
					{
						id: 'remove',
						label: 'Remove',
						icon: 'cross' as const,
						action: async () => {
							const id = actionsUserId!;
							actionsAnchor = null;
							actionsUserId = null;
							await removeUser(id);
						}
					}
				]}
				width={160}
				onClose={() => {
					actionsAnchor = null;
					actionsUserId = null;
				}}
			/>
		</FloatingBox>
	</Portal>
{/if}

{#if addUserAnchor}
	<Portal>
		<FloatingBox anchor={addUserAnchor} backdrop onBackdropClick={closeAddUserMenu}>
			<Menu
				options={addUserMenuOptions}
				searchEnabled={true}
				searchPlaceholder="Enter exact email…"
				searchValidator={(v) => (v && !isEmail(v) ? 'Enter a valid email' : null)}
				bind:searchQuery={addUserSearch}
				externalFilter={true}
				loading={addUserLoading}
				emptyMessage="Type an email address"
				noResultsMessage="Type an email address"
				width={300}
				onClose={closeAddUserMenu}
			/>
		</FloatingBox>
	</Portal>
{/if}

<style>
	.section {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-xs);
		padding: var(--space-sm);
		flex-shrink: 0;
		height: 24px;
	}

	.users-panel {
		display: flex;
		flex-direction: column;
		height: 100%;
		overflow: hidden;
		width: 100%;
	}

	.cell-user {
		display: flex;
		align-items: center;
		gap: var(--space-xs-sm);
	}

	.user-name {
		color: var(--gray-900);
	}

	.text-muted {
		color: var(--gray-800);
	}

	.cell-roles {
		display: flex;
		align-items: center;
		gap: var(--space-xs);
	}

	.users-table-wrap {
		flex: 1;
		overflow: hidden;
		margin: 0 var(--space-sm);
	}

	.actions-inner {
		display: flex;
		align-items: center;
		justify-content: flex-end;
	}

	.action-btn-wrap {
		visibility: hidden;
	}

	:global(.users-table-wrap tr:hover) .action-btn-wrap {
		visibility: visible;
	}

	.badge-owner {
		font-size: var(--fs-xs);
		font-weight: var(--fw-md);
		padding: var(--space-xxs) var(--space-xs);
		border-radius: var(--br-xs);
		border: var(--border);
		color: var(--gray-900);
		white-space: nowrap;
		flex-shrink: 0;
	}
</style>
