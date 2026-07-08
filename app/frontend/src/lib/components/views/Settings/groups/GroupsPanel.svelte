<script lang="ts">
	import type { Component } from 'svelte';
	import Button from '$lib/system/Button/Button.svelte';
	import Icon from '$lib/system/Icon/Icon.svelte';
	import Table from '$lib/system/Table/Table.svelte';
	import { must, tryCatch } from '$lib/utils/tryCatch';
	import { modalStore } from '$lib/system/Modal/ModalStore';
	import { ListGroups, CreateGroup, DeleteGroup, RenameGroup } from '$lib/wailsjs/go/group/Group';
	import type { generated } from '$lib/wailsjs/go/models';
	import ConfirmDeleteGroupModal from './ConfirmDeleteGroupModal.svelte';
	import CreateGroupModal from './CreateGroupModal.svelte';
	import RenameGroupModal from './RenameGroupModal.svelte';
	import GroupMembersPicker from './GroupMembersPicker.svelte';
	import GroupRolesPicker from './GroupRolesPicker.svelte';
	import FloatingBox from '$lib/system/FloatingBox/FloatingBox.svelte';
	import Menu from '$lib/system/Menu/Menu.svelte';
	import Portal from '$lib/system/Portal/Portal.svelte';
	import { EventsOn, EventsOff } from '$lib/wailsjs/runtime/runtime';

	let groups = $state<generated.ListGroupsByWorkspaceRow[]>([]);
	let selectedGroupId = $state<string | null>(null);
	let menuAnchorEl = $state<HTMLElement | null>(null);
	let menuGroupId = $state<string | null>(null);

	let menuGroup = $derived(groups.find((g) => g.id === menuGroupId) ?? null);
	let selectedGroup = $derived(groups.find((g) => g.id === selectedGroupId) ?? null);

	function selectGroup(id: string | null) {
		selectedGroupId = id;
		if (!id) load();
	}

	async function load() {
		const result = await must(tryCatch(ListGroups));
		groups = (result as generated.ListGroupsByWorkspaceRow[]) ?? [];
	}

	function openCreateModal() {
		modalStore.set({
			content: (() => CreateGroupModal) as () => Component,
			props: {
				onClose: () => modalStore.set(null),
				onCreate: async (name: string) => {
					const created = await must(tryCatch(CreateGroup, name));
					await load();
					selectGroup((created as { id: string }).id);
				}
			},
			width: 400
		});
	}

	function openRenameModal(grp: generated.ListGroupsByWorkspaceRow) {
		modalStore.set({
			content: (() => RenameGroupModal) as () => Component,
			props: {
				currentName: grp.name,
				onClose: () => modalStore.set(null),
				onRename: async (name: string) => {
					await must(tryCatch(RenameGroup, grp.id, name));
					await load();
				}
			},
			width: 400
		});
	}

	function openDeleteConfirm(grp: generated.ListGroupsByWorkspaceRow) {
		modalStore.set({
			content: (() => ConfirmDeleteGroupModal) as () => Component,
			props: {
				groupName: grp.name,
				onClose: () => modalStore.set(null),
				onConfirm: async () => {
					await must(tryCatch(DeleteGroup, grp.id));
					groups = groups.filter((g) => g.id !== grp.id);
					if (selectedGroupId === grp.id) selectGroup(null);
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

{#if selectedGroup}
	<div class="detail-view">
		<div class="detail-header">
			<button class="back-btn" onclick={() => selectGroup(null)}>
				<Icon icon="chevron-left" size={14} />
				<span>Groups</span>
			</button>
			<span class="breadcrumb-sep">/</span>
			<span class="breadcrumb-group">{selectedGroup.name}</span>
			<span class="rename-btn-wrap">
				<Button
					leftIcon="edit"
					iconSize={13}
					emphasis="low"
					size="sm"
					onclick={() => openRenameModal(selectedGroup)}
				/>
			</span>
			<div class="header-members">
				<GroupMembersPicker groupId={selectedGroup.id} />
			</div>
		</div>
		<GroupRolesPicker groupId={selectedGroup.id} />
	</div>
{:else}
	<div class="list-view">
		<div class="section">
			<p>Groups</p>
			<Button
				content="New group"
				emphasis="high"
				size="sm"
				onclick={openCreateModal}
				style="height: 22px; padding: 0 var(--space-xs);"
			/>
		</div>
		<div class="group-table-wrap">
			<Table
				columns={[
					{ key: 'name', label: 'Name', searchable: true, width: '200px', pinned: true },
					{ key: 'members', label: 'Members', width: '160px' },
					{ key: 'roles', label: 'Roles', align: 'right', width: '110px' },
					{ key: 'actions', width: '52px' }
				]}
				rows={groups}
				getKey={(g) => g.id}
				filterValue={(g) => g.name}
				onRowClick={(g) => selectGroup(g.id)}
				{cell}
			/>
		</div>
	</div>
{/if}

{#snippet cell(key: string, grp: generated.ListGroupsByWorkspaceRow)}
	{#if key === 'name'}
		<span>{grp.name}</span>
	{:else if key === 'members'}
		<span role="none" onclick={(e) => e.stopPropagation()}>
			<GroupMembersPicker groupId={grp.id} />
		</span>
	{:else if key === 'roles'}
		<span>{grp.role_count}</span>
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
						menuGroupId = menuGroupId === grp.id ? null : grp.id;
					}}
				/>
			</span>
		</div>
	{/if}
{/snippet}

{#if menuGroupId && menuAnchorEl && menuGroup}
	<Portal>
		<FloatingBox anchor={menuAnchorEl} backdrop onBackdropClick={() => (menuGroupId = null)}>
			<Menu
				width={160}
				options={[
					{
						id: 'rename',
						label: 'Rename',
						action: () => {
							const grp = menuGroup!;
							menuGroupId = null;
							openRenameModal(grp);
						}
					},
					{ id: 'divider', label: '', type: 'divider' },
					{
						id: 'delete',
						label: 'Delete',
						action: () => {
							const grp = menuGroup!;
							menuGroupId = null;
							openDeleteConfirm(grp);
						}
					}
				]}
				onClose={() => (menuGroupId = null)}
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

	.group-table-wrap {
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

	:global(.group-table-wrap tr:hover) .action-btn-wrap {
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

	.breadcrumb-group {
		font-size: var(--fs-sm);
		color: var(--gray-900);
	}
</style>
