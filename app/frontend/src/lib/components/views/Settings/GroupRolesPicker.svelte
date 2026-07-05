<script lang="ts">
	import { tryCatch } from '$lib/utils/tryCatch';
	import { notify } from '$lib/system/Notifications/notificationsStore';
	import { AlertType } from '$lib/system/Alert/types';
	import {
		ListGroupRoles,
		AttachRoleToGroup,
		DetachRoleFromGroup
	} from '$lib/wailsjs/go/group/Group';
	import { ListRoles } from '$lib/wailsjs/go/role/Role';
	import { EventsOn, EventsOff } from '$lib/wailsjs/runtime/runtime';
	import FloatingBox from '$lib/system/FloatingBox/FloatingBox.svelte';
	import Menu from '$lib/system/Menu/Menu.svelte';
	import Portal from '$lib/system/Portal/Portal.svelte';
	import Button from '$lib/system/Button/Button.svelte';
	import type { group, generated } from '$lib/wailsjs/go/models';
	import type { MenuOption } from '$lib/system/Menu/Menu.types';

	type Props = { groupId: string };
	let { groupId }: Props = $props();

	let attached = $state<group.GroupRoleEntry[]>([]);
	let wsRoles = $state<generated.ListRolesByWorkspaceRow[]>([]);
	let menuAnchor = $state<HTMLElement | null>(null);
	let menuOpen = $state(false);
	let search = $state('');

	async function loadAttached() {
		const [r, err] = await tryCatch(ListGroupRoles, groupId);
		if (!err) attached = (r as group.GroupRoleEntry[]) ?? [];
	}

	async function loadWsRoles() {
		const [r, err] = await tryCatch(ListRoles);
		if (!err) wsRoles = (r as generated.ListRolesByWorkspaceRow[]) ?? [];
	}

	$effect(() => {
		void groupId;
		loadAttached();
		loadWsRoles();
	});

	$effect(() => {
		EventsOn('rolesUpdated', () => {
			loadAttached();
			loadWsRoles();
		});
		return () => EventsOff('rolesUpdated');
	});

	let attachedIds = $derived(new Set(attached.map((r) => r.id)));

	async function attach(roleId: string) {
		const [, err] = await tryCatch(AttachRoleToGroup, roleId, groupId);
		if (err) {
			notify({ type: AlertType.Error, message: err?.message ?? 'Failed to attach role' });
			return;
		}
		await loadAttached();
	}

	async function detach(groupToRoleId: string) {
		const [, err] = await tryCatch(DetachRoleFromGroup, groupToRoleId);
		if (err) {
			notify({ type: AlertType.Error, message: err?.message ?? 'Failed to detach role' });
			return;
		}
		await loadAttached();
	}

	let options = $derived.by<MenuOption<string>[]>(() => {
		const q = search.trim().toLowerCase();
		return wsRoles
			.filter((r) => !q || r.name.toLowerCase().includes(q))
			.map((r) => ({
				id: r.id,
				label: r.name,
				checkbox: true,
				checked: attachedIds.has(r.id),
				action: (opt) => toggle(opt.id),
				onCheckClick: (opt) => toggle(opt.id)
			}));
	});

	function toggle(roleId: string) {
		if (attachedIds.has(roleId)) {
			const match = attached.find((r) => r.id === roleId);
			if (match) detach(match.group_to_role_id);
		} else {
			attach(roleId);
		}
	}

	function openMenu(anchor: HTMLElement) {
		search = '';
		menuAnchor = anchor;
		menuOpen = true;
	}
</script>

<div class="roles-panel">
	<div class="roles-head">
		<span class="roles-title">Roles granted to members</span>
		<Button
			content="Attach role"
			emphasis="low"
			size="sm"
			leftIcon="plus"
			onclick={(e) => openMenu(e.currentTarget as HTMLElement)}
		/>
	</div>

	{#if attached.length === 0}
		<p class="empty">No roles attached. Members inherit no roles from this group yet.</p>
	{:else}
		<ul class="role-list">
			{#each attached as r (r.group_to_role_id)}
				<li class="role-row">
					<span class="role-name">{r.name ?? r.id}</span>
					<span class="remove-wrap">
						<Button
							leftIcon="cross"
							iconSize={13}
							emphasis="low"
							size="sm"
							onclick={() => detach(r.group_to_role_id)}
						/>
					</span>
				</li>
			{/each}
		</ul>
	{/if}
</div>

{#if menuOpen && menuAnchor}
	<Portal>
		<FloatingBox anchor={menuAnchor} backdrop onBackdropClick={() => (menuOpen = false)}>
			<Menu
				{options}
				searchEnabled={true}
				searchPlaceholder="Search roles…"
				bind:searchQuery={search}
				externalFilter={true}
				emptyMessage="No roles in this workspace"
				noResultsMessage="No roles match"
				width={240}
				onClose={() => (menuOpen = false)}
			/>
		</FloatingBox>
	</Portal>
{/if}

<style>
	.roles-panel {
		display: flex;
		flex-direction: column;
		gap: var(--space-sm);
		padding: var(--space-md);
		overflow: auto;
	}

	.roles-head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-sm);
	}

	.roles-title {
		font-size: var(--fs-sm);
		color: var(--gray-900);
	}

	.empty {
		font-size: var(--fs-sm);
		color: var(--gray-700);
		text-wrap: wrap;
	}

	.role-list {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.role-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: var(--space-xs) var(--space-sm);
		border: var(--border);
		border-radius: var(--radius-sm);
	}

	.role-name {
		font-size: var(--fs-sm);
		color: var(--gray-900);
	}

	.remove-wrap {
		visibility: hidden;
	}

	.role-row:hover .remove-wrap {
		visibility: visible;
	}
</style>
