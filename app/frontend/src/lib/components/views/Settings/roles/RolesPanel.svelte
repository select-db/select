<script lang="ts">
	import type { Component } from 'svelte';
	import Button from '$lib/system/Button/Button.svelte';
	import Icon from '$lib/system/Icon/Icon.svelte';
	import Table from '$lib/system/Table/Table.svelte';
	import { must, tryCatch } from '$lib/utils/tryCatch';
	import { modalStore } from '$lib/system/Modal/ModalStore';
	import {
		ListRoles,
		CreateRole,
		DeleteRole,
		RenameRole,
		DuplicateRole
	} from '$lib/wailsjs/go/role/Role';
	import type { generated } from '$lib/wailsjs/go/models';
	import ConfirmDeleteRoleModal from './ConfirmDeleteRoleModal.svelte';
	import CreateRoleModal from './CreateRoleModal.svelte';
	import RenameRoleModal from './RenameRoleModal.svelte';
	import RoleDetail from './RoleDetail.svelte';
	import RoleMembersPicker from './RoleMembersPicker.svelte';
	import {
		getActiveTab,
		updateSettingsTab,
		layoutStore,
		type Tab
	} from '$lib/components/Layout/layoutStore';
	import { debounce } from '$lib/utils/debounce';
	import FloatingBox from '$lib/system/FloatingBox/FloatingBox.svelte';
	import Menu from '$lib/system/Menu/Menu.svelte';
	import Portal from '$lib/system/Portal/Portal.svelte';
	import { EventsOn, EventsOff } from '$lib/wailsjs/runtime/runtime';

	const saved = getActiveTab()?.settings?.roles;

	let roles = $state<generated.ListRolesByWorkspaceRow[]>([]);
	let selectedRoleId = $state<string | null>(saved?.selectedRoleId ?? null);

	// Sync selectedRoleId when the store resets it
	$effect(() => {
		const storeRoleId = getActiveTab()?.settings?.roles?.selectedRoleId ?? null;
		void $layoutStore;
		if (storeRoleId !== selectedRoleId) selectedRoleId = storeRoleId;
	});
	let menuAnchorEl = $state<HTMLElement | null>(null);
	let menuRoleId = $state<string | null>(null);

	let menuRole = $derived(roles.find((r) => r.id === menuRoleId) ?? null);

	function selectRole(id: string | null) {
		selectedRoleId = id;
		const name = id ? (roles.find((r) => r.id === id)?.name ?? null) : null;
		updateSettingsTab({ roles: { ...saved, selectedRoleId: id, selectedRoleName: name } });
		if (!id) load();
	}

	let selectedRole = $derived(roles.find((r) => r.id === selectedRoleId) ?? null);

	const onRoleDetailStateChange = debounce(
		(s: NonNullable<NonNullable<Tab['settings']>['roles']>) => {
			if (!selectedRole) return;
			updateSettingsTab({
				roles: { selectedRoleId: selectedRole.id, selectedRoleName: selectedRole.name, ...s }
			});
		},
		300
	);

	async function load() {
		const result = await must(tryCatch(ListRoles));
		roles = (result as generated.ListRolesByWorkspaceRow[]) ?? [];
	}

	function openCreateModal() {
		modalStore.set({
			content: (() => CreateRoleModal) as () => Component,
			props: {
				onClose: () => modalStore.set(null),
				onCreate: async (name: string) => {
					const created = await must(tryCatch(CreateRole, name));
					await load();
					selectRole((created as { id: string }).id);
				}
			},
			width: 400
		});
	}

	function openRenameModal(role: generated.ListRolesByWorkspaceRow) {
		modalStore.set({
			content: (() => RenameRoleModal) as () => Component,
			props: {
				currentName: role.name,
				onClose: () => modalStore.set(null),
				onRename: async (name: string) => {
					await must(tryCatch(RenameRole, role.id, name));
					await load();
				}
			},
			width: 400
		});
	}

	async function duplicateRole(role: generated.ListRolesByWorkspaceRow) {
		const created = await must(tryCatch(DuplicateRole, role.id));
		await load();
		selectRole((created as { id: string }).id);
	}

	function openDeleteConfirm(role: generated.ListRolesByWorkspaceRow) {
		modalStore.set({
			content: (() => ConfirmDeleteRoleModal) as () => Component,
			props: {
				roleName: role.name,
				onClose: () => modalStore.set(null),
				onConfirm: async () => {
					await must(tryCatch(DeleteRole, role.id));
					roles = roles.filter((r) => r.id !== role.id);
					if (selectedRoleId === role.id) selectRole(null);
				}
			},
			width: 400
		});
	}

	$effect(() => {
		load();
	});

	$effect(() => {
		EventsOn('rolesUpdated', load);
		return () => EventsOff('rolesUpdated');
	});
</script>

{#if selectedRole}
	<div class="detail-view">
		<div class="detail-header">
			<button class="back-btn" onclick={() => selectRole(null)}>
				<Icon icon="chevron-left" size={14} />
				<span>Roles</span>
			</button>
			<span class="breadcrumb-sep">/</span>
			<span class="breadcrumb-role">{selectedRole.name}</span>
			<span class="rename-btn-wrap">
				<Button
					leftIcon="edit"
					iconSize={13}
					emphasis="low"
					size="sm"
					onclick={() => openRenameModal(selectedRole)}
				/>
			</span>
			<div class="header-members">
				<RoleMembersPicker roleId={selectedRole.id} />
			</div>
		</div>
		<RoleDetail
			roleId={selectedRole.id}
			savedState={saved}
			onStateChange={onRoleDetailStateChange}
		/>
	</div>
{:else}
	<div class="list-view">
		<div class="section">
			<p>Roles</p>
			<Button
				content="New role"
				emphasis="high"
				size="sm"
				onclick={openCreateModal}
				style="height: 22px; padding: 0 var(--space-xs);"
			/>
		</div>
		<div class="role-table-wrap">
			<Table
				columns={[
					{ key: 'name', label: 'Name', searchable: true, width: '200px', pinned: true },
					{ key: 'users', label: 'Users', width: '160px' },
					{ key: 'permissions', label: 'Permissions', align: 'right', width: '110px' },
					{ key: 'actions', width: '52px' }
				]}
				rows={roles}
				getKey={(r) => r.id}
				filterValue={(r) => r.name}
				onRowClick={(r) => selectRole(r.id)}
				{cell}
			/>
		</div>
	</div>
{/if}

{#snippet cell(key: string, role: generated.ListRolesByWorkspaceRow)}
	{#if key === 'name'}
		<span>{role.name}</span>
	{:else if key === 'users'}
		<span role="none" onclick={(e) => e.stopPropagation()}>
			<RoleMembersPicker roleId={role.id} />
		</span>
	{:else if key === 'permissions'}
		<span>{role.permission_count}</span>
	{:else if key === 'actions'}
		<div class="actions-inner">
			<span class="action-btn-wrap">
				<Button
					leftIcon="dots"
					iconSize={14}
					emphasis="low"
					size="sm"
					onclick={(e) => {
						e.stopPropagation();
						menuAnchorEl = e.currentTarget as HTMLElement;
						menuRoleId = menuRoleId === role.id ? null : role.id;
					}}
				/>
			</span>
		</div>
	{/if}
{/snippet}

{#if menuRoleId && menuAnchorEl && menuRole}
	<Portal>
		<FloatingBox anchor={menuAnchorEl} backdrop onBackdropClick={() => (menuRoleId = null)}>
			<Menu
				width={160}
				options={[
					{
						id: 'rename',
						label: 'Rename',
						action: () => {
							const role = menuRole!;
							menuRoleId = null;
							openRenameModal(role);
						}
					},
					{
						id: 'duplicate',
						label: 'Duplicate',
						action: () => {
							const role = menuRole!;
							menuRoleId = null;
							duplicateRole(role);
						}
					},
					{ id: 'divider', label: '', type: 'divider' },
					{
						id: 'delete',
						label: 'Delete',
						action: () => {
							const role = menuRole!;
							menuRoleId = null;
							openDeleteConfirm(role);
						}
					}
				]}
				onClose={() => (menuRoleId = null)}
			/>
		</FloatingBox>
	</Portal>
{/if}

<style>
	.list-view {
		display: flex;
		flex-direction: column;
		height: 100%;
		overflow: hidden;
		width: 100%;
	}

	.section {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-xs);
		padding: var(--space-sm);
		flex-shrink: 0;
		height: 24px;
	}

	.role-table-wrap {
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

	:global(.role-table-wrap tr:hover) .action-btn-wrap {
		visibility: visible;
	}

	.detail-view {
		display: flex;
		flex-direction: column;
		height: 100%;
		overflow: hidden;
	}

	.detail-header {
		display: flex;
		align-items: center;
		height: 24px;
		gap: var(--space-xs);
		padding: var(--space-sm);
		border-bottom: var(--border);
		flex-shrink: 0;
	}

	.rename-btn-wrap {
		display: contents;
		visibility: hidden;
	}
	.detail-header:hover .rename-btn-wrap {
		visibility: visible;
	}

	.header-members {
		margin-left: auto;
	}

	.back-btn {
		display: flex;
		align-items: center;
		gap: var(--space-xxs);
		background: none;
		border: none;
		cursor: pointer;
		font-size: var(--fs-sm);
		padding: 0;
	}
	.back-btn > span {
		color: var(--gray-800);
	}
	.back-btn:hover > span {
		color: var(--gray-1000);
		text-decoration: underline;
	}
	.breadcrumb-sep {
		color: var(--gray-600);
		font-size: var(--fs-sm);
	}

	.breadcrumb-role {
		font-size: var(--fs-sm);
		color: var(--gray-900);
	}
</style>
