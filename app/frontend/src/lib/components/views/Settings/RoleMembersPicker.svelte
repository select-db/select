<script lang="ts">
	import Avatar from '$lib/system/Avatar/Avatar.svelte';
	import { tryCatch } from '$lib/utils/tryCatch';
	import { notify } from '$lib/system/Notifications/notificationsStore';
	import { AlertType } from '$lib/system/Alert/types';
	import { ListUsersInRole, AssignUserToRole, RemoveUserFromRole } from '$lib/wailsjs/go/role/Role';
	import { ListWorkspaceUsers } from '$lib/wailsjs/go/workspace/Workspace';
	import { GetCurrentUserAvatar } from '$lib/wailsjs/go/user/User';
	import { EventsOn, EventsOff } from '$lib/wailsjs/runtime/runtime';
	import FloatingBox from '$lib/system/FloatingBox/FloatingBox.svelte';
	import Menu from '$lib/system/Menu/Menu.svelte';
	import Portal from '$lib/system/Portal/Portal.svelte';
	import Button from '$lib/system/Button/Button.svelte';
	import type { role, workspace } from '$lib/wailsjs/go/models';
	import type { MenuOption, MenuOptionContentSnippet } from '$lib/system/Menu/Menu.types';

	const MAX_VISIBLE = 5;

	type Props = { roleId: string };
	let { roleId }: Props = $props();

	let roleUsers = $state<role.RoleUserEntry[]>([]);
	let wsUsers = $state<workspace.WorkspaceUserEntry[]>([]);
	let avatars = $state<Record<string, string>>({});
	let menuAnchor = $state<HTMLElement | null>(null);
	let menuOpen = $state(false);
	let search = $state('');

	async function loadRoleUsers() {
		const [r, err] = await tryCatch(ListUsersInRole, roleId);
		if (!err) roleUsers = (r as role.RoleUserEntry[]) ?? [];
	}

	async function loadWsUsers() {
		const [r, err] = await tryCatch(ListWorkspaceUsers);
		if (!err) wsUsers = (r as workspace.WorkspaceUserEntry[]) ?? [];
	}

	async function loadAvatars(users: { id: string }[]) {
		for (const u of users) {
			if (avatars[u.id]) continue;
			const [src] = await tryCatch(GetCurrentUserAvatar, u.id);
			if (src) avatars = { ...avatars, [u.id]: src as string };
		}
	}

	$effect(() => {
		void roleId;
		loadRoleUsers();
		loadWsUsers();
	});

	$effect(() => {
		if (roleUsers.length > 0 || wsUsers.length > 0) {
			loadAvatars([...roleUsers, ...wsUsers]);
		}
	});

	$effect(() => {
		EventsOn('rolesUpdated', () => {
			loadRoleUsers();
			loadWsUsers();
		});
		return () => EventsOff('rolesUpdated');
	});

	type UserOptionData = { user: workspace.WorkspaceUserEntry; avatar: string | undefined };

	let visibleUsers = $derived(roleUsers.slice(0, MAX_VISIBLE));
	let overflow = $derived(Math.max(0, roleUsers.length - MAX_VISIBLE));
	let assignedIds = $derived(new Set(roleUsers.map((u) => u.id)));

	async function toggleUser(userId: string) {
		if (assignedIds.has(userId)) {
			const match = roleUsers.find((r) => r.id === userId);
			if (!match) return;
			const [, err] = await tryCatch(RemoveUserFromRole, match.user_to_role_id);
			if (err) {
				notify({ type: AlertType.Error, message: err?.message ?? 'Failed to remove user' });
				return;
			}
		} else {
			const [, err] = await tryCatch(AssignUserToRole, userId, roleId);
			if (err) {
				notify({ type: AlertType.Error, message: err?.message ?? 'Failed to assign user' });
				return;
			}
		}
		await loadRoleUsers();
	}

	let allOptions = $derived<MenuOption<string>[]>(
		wsUsers.map((u) => ({
			id: u.id,
			label: u.name ?? u.id,
			checkbox: true,
			checked: assignedIds.has(u.id),
			customData: { user: u, avatar: avatars[u.id] } satisfies UserOptionData,
			action: (opt) => toggleUser(opt.id),
			onCheckClick: (opt) => toggleUser(opt.id)
		}))
	);

	let filteredOptions = $derived.by<MenuOption<string>[]>(() => {
		const q = search.trim().toLowerCase();
		if (!q) return allOptions;
		return allOptions.filter((o) => {
			const data = o.customData as UserOptionData;
			return (
				(data.user.name ?? '').toLowerCase().includes(q) ||
				(data.user.email ?? '').toLowerCase().includes(q)
			);
		});
	});

	function openMenu(anchor: HTMLElement) {
		search = '';
		menuAnchor = anchor;
		menuOpen = true;
	}
</script>

{#snippet userOptionContent(data: unknown)}
	{@const { user, avatar } = data as UserOptionData}
	<div class="option-user">
		<Avatar name={user.name} src={avatar} size={16} />
		<span class="option-info">
			<span class="option-name">{user.name ?? user.id}</span>
			{#if user.email}
				<span class="option-email truncate">{user.email}</span>
			{/if}
		</span>
	</div>
{/snippet}

<div class="role-members">
	{#if roleUsers.length === 0}
		<Button
			content="Add user"
			emphasis="low"
			size="sm"
			style="height: 26px"
			onclick={(e) => openMenu(e.currentTarget as HTMLElement)}
		/>
	{:else}
		<button class="badge-strip" onclick={(e) => openMenu(e.currentTarget as HTMLElement)}>
			{#each visibleUsers as u (u.user_to_role_id)}
				<span class="badge" title={u.name ?? u.id}>
					<Avatar name={u.name} src={avatars[u.id]} size={18} />
				</span>
			{/each}
			{#if overflow > 0}
				<span class="badge badge-overflow">+{overflow}</span>
			{/if}
		</button>
	{/if}
</div>

{#if menuOpen && menuAnchor}
	<Portal>
		<FloatingBox anchor={menuAnchor} backdrop onBackdropClick={() => (menuOpen = false)}>
			<Menu
				options={filteredOptions}
				searchEnabled={true}
				searchPlaceholder="Search by name or email…"
				bind:searchQuery={search}
				externalFilter={true}
				emptyMessage="No workspace users"
				noResultsMessage="No users match"
				width={260}
				optionContent={userOptionContent as MenuOptionContentSnippet}
				onClose={() => (menuOpen = false)}
			/>
		</FloatingBox>
	</Portal>
{/if}

<style>
	.role-members {
		display: flex;
		align-items: center;
	}

	.badge-strip {
		display: flex;
		align-items: center;
		gap: 2px;
		background: none;
		border: none;
		cursor: pointer;
		padding: 0;
	}

	.badge {
		width: 18px;
		height: 18px;
		border-radius: 50%;
		background: var(--gray-200);
		color: var(--gray-900);
		font-size: 9px;
		font-weight: 600;
		display: flex;
		align-items: center;
		justify-content: center;
		overflow: hidden;
		border: 1px solid var(--gray-100);
		flex-shrink: 0;
		letter-spacing: 0;
		margin-left: -8px;
		user-select: none;
	}

	.badge:first-child {
		margin-left: 0;
	}

	.badge-overflow {
		background: var(--gray-600);
		color: var(--gray-800);
	}

	.badge-strip:hover .badge {
		border-color: var(--gray-1000);
	}

	.option-user {
		display: flex;
		align-items: center;
		gap: var(--space-xs-sm);
		min-width: 0;
	}

	.option-info {
		display: flex;
		gap: var(--space-xs);
		align-items: center;
		min-width: 0;
	}

	.option-name {
		font-size: var(--fs-sm);
		color: var(--gray-900);
	}

	.option-email {
		font-size: var(--fs-xs);
		color: var(--gray-800) !important;
	}
</style>
