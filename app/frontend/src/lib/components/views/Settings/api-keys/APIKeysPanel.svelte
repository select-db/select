<script lang="ts">
	import type { Component } from 'svelte';
	import Table from '$lib/system/Table/Table.svelte';
	import Button from '$lib/system/Button/Button.svelte';
	import Menu from '$lib/system/Menu/Menu.svelte';
	import Portal from '$lib/system/Portal/Portal.svelte';
	import FloatingBox from '$lib/system/FloatingBox/FloatingBox.svelte';
	import Alert from '$lib/system/Alert/Alert.svelte';
	import { AlertType } from '$lib/system/Alert/types';
	import { modalStore } from '$lib/system/Modal/ModalStore';
	import { must, tryCatch } from '$lib/utils/tryCatch';
	import { myPermissions } from '$lib/stores/myPermissionsStore';
	import { formatRelativeTime, formatExpiry } from '$lib/utils/formatRelativeTime';
	import {
		ListAPIKeys,
		CreateAPIKey,
		RevokeAPIKey,
		RotateAPIKey,
		SetAPIKeyRoles
	} from '$lib/wailsjs/go/apikey/APIKey';
	import { ListRoles } from '$lib/wailsjs/go/role/Role';
	import type { apikey, generated } from '$lib/wailsjs/go/models';
	import CreateAPIKeyModal from './CreateAPIKeyModal.svelte';
	import RevealAPIKeyModal from './RevealAPIKeyModal.svelte';
	import ConfirmRevokeAPIKeyModal from './ConfirmRevokeAPIKeyModal.svelte';
	import ConfirmRotateAPIKeyModal from './ConfirmRotateAPIKeyModal.svelte';

	type Key = apikey.APIKeyEntry;

	const columns = [
		{ key: 'name', label: 'Name', searchable: true, width: '200px', pinned: true },
		{ key: 'key', label: 'Key', width: '170px' },
		{ key: 'roles', label: 'Roles', width: '200px' },
		{ key: 'last_used', label: 'Last used', width: '110px' },
		{ key: 'expires', label: 'Expires', width: '150px' },
		{ key: 'actions', label: '', width: '52px' }
	];

	let keys = $state<Key[]>([]);
	let roles = $state<{ id: string; name: string }[]>([]);
	let loadError = $state<string | null>(null);
	let menuAnchor = $state<HTMLElement | null>(null);
	let menuKeyId = $state<string | null>(null);
	let roleAnchor = $state<HTMLElement | null>(null);
	let roleKeyId = $state<string | null>(null);
	let roleSearch = $state('');

	let canManage = $derived($myPermissions.isAllowed('workspace/api-keys.manage'));
	let menuKey = $derived(keys.find((k) => k.id === menuKeyId) ?? null);
	let roleKey = $derived(keys.find((k) => k.id === roleKeyId) ?? null);
	let assignedRoleIds = $derived(new Set(roleKey?.roles?.map((r) => r.id) ?? []));

	async function load() {
		const [k, keysErr] = await tryCatch(ListAPIKeys);
		if (keysErr) {
			loadError = keysErr.message;
			return;
		}
		const [r, rolesErr] = await tryCatch(ListRoles);
		if (rolesErr) {
			loadError = rolesErr.message;
			return;
		}
		loadError = null;
		keys = (k as Key[]) ?? [];
		roles = ((r as generated.ListRolesByWorkspaceRow[]) ?? []).map((x) => ({
			id: x.id,
			name: x.name
		}));
	}

	$effect(() => {
		load();
	});

	function openReveal(result: apikey.CreateResult) {
		modalStore.set({
			content: (() => RevealAPIKeyModal) as () => Component,
			props: { onClose: () => modalStore.set(null), apiKey: result.key },
			width: 460
		});
	}

	function openCreate() {
		modalStore.set({
			content: (() => CreateAPIKeyModal) as () => Component,
			props: {
				onClose: () => modalStore.set(null),
				roles,
				onCreate: async (params: apikey.CreateParams) => {
					const res = await must(tryCatch(CreateAPIKey, params));
					await load();
					openReveal(res as apikey.CreateResult);
				}
			},
			width: 420
		});
	}

	function rotate(key: Key) {
		closeMenu();
		modalStore.set({
			content: (() => ConfirmRotateAPIKeyModal) as () => Component,
			props: {
				onClose: () => modalStore.set(null),
				keyName: key.name ?? key.id,
				onConfirm: async () => {
					const res = await must(tryCatch(RotateAPIKey, key.id));
					await load();
					openReveal(res as apikey.CreateResult);
				}
			},
			width: 400
		});
	}

	function revoke(key: Key) {
		closeMenu();
		modalStore.set({
			content: (() => ConfirmRevokeAPIKeyModal) as () => Component,
			props: {
				onClose: () => modalStore.set(null),
				keyName: key.name ?? key.id,
				onConfirm: async () => {
					await must(tryCatch(RevokeAPIKey, key.id));
					await load();
				}
			},
			width: 400
		});
	}

	function openMenu(anchor: HTMLElement, id: string) {
		menuAnchor = anchor;
		menuKeyId = id;
	}
	function closeMenu() {
		menuAnchor = null;
		menuKeyId = null;
	}

	function openRoleMenu(anchor: HTMLElement, id: string) {
		roleSearch = '';
		roleAnchor = anchor;
		roleKeyId = id;
	}
	function closeRoleMenu() {
		roleAnchor = null;
		roleKeyId = null;
	}

	async function toggleRole(roleId: string) {
		if (!roleKey) return;
		// eslint-disable-next-line svelte/prefer-svelte-reactivity -- local set, spread into an array argument and discarded
		const next = new Set(assignedRoleIds);
		if (next.has(roleId)) next.delete(roleId);
		else next.add(roleId);
		await must(tryCatch(SetAPIKeyRoles, roleKey.id, [...next]));
		await load();
	}

	let roleMenuOptions = $derived(
		roles.map((r) => ({
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

	let menuOptions = $derived(
		menuKey
			? [
					{
						id: 'rotate',
						label: 'Rotate',
						icon: 'refresh' as const,
						action: () => rotate(menuKey!)
					},
					{ id: 'revoke', label: 'Revoke', icon: 'cross' as const, action: () => revoke(menuKey!) }
				]
			: []
	);
</script>

<div class="apikeys-panel">
	<div class="section">
		<p>API keys</p>
		{#if canManage && !loadError}
			<Button
				content="New API key"
				emphasis="high"
				size="sm"
				onclick={openCreate}
				style="height: 22px; padding: 0 var(--space-xs);"
			/>
		{/if}
	</div>
	{#if loadError}
		<div class="error-container">
			<Alert type={AlertType.Error} message={loadError} noPulse />
		</div>
	{:else}
		<div class="keys-table-wrap">
			<Table {columns} rows={keys} getKey={(k) => k.id} filterValue={(k) => k.name ?? ''} {cell} />
		</div>
	{/if}
</div>

{#snippet cell(key: string, k: Key)}
	{#if key === 'name'}
		<span class="name selectable truncate">{k.name ?? k.id}</span>
	{:else if key === 'key'}
		<code class="prefix selectable">slct_{k.prefix ? k.prefix + '*****' : '-'}</code>
	{:else if key === 'roles'}
		{#if canManage}
			<Button
				content={k.roles.length ? k.roles.map((r) => r.name).join(', ') : 'Add role'}
				emphasis="low"
				size="sm"
				truncate
				onclick={(e) => openRoleMenu(e.currentTarget as HTMLElement, k.id)}
			/>
		{:else}
			<span class="text-muted truncate">{k.roles.map((r) => r.name).join(', ') || '-'}</span>
		{/if}
	{:else if key === 'last_used'}
		<span class="text-muted">{formatRelativeTime(k.last_used_at)}</span>
	{:else if key === 'expires'}
		<span class="text-muted">{formatExpiry(k.expires_at)}</span>
	{:else if key === 'actions'}
		{#if canManage}
			<div class="actions-inner">
				<span class="action-btn-wrap">
					<Button
						leftIcon="dots"
						iconSize={14}
						emphasis="low"
						size="sm"
						onclick={(e) => {
							e.stopPropagation();
							openMenu(e.currentTarget as HTMLElement, k.id);
						}}
					/>
				</span>
			</div>
		{/if}
	{/if}
{/snippet}

{#if menuAnchor && menuKey}
	<Portal>
		<FloatingBox anchor={menuAnchor} backdrop onBackdropClick={closeMenu}>
			<Menu options={menuOptions} width={160} onClose={closeMenu} />
		</FloatingBox>
	</Portal>
{/if}

{#if roleAnchor && roleKey}
	<Portal>
		<FloatingBox anchor={roleAnchor} backdrop onBackdropClick={closeRoleMenu}>
			<Menu
				options={filteredRoleOptions}
				searchEnabled={true}
				searchPlaceholder="Search roles…"
				bind:searchQuery={roleSearch}
				externalFilter={true}
				emptyMessage="No roles"
				noResultsMessage="No matching roles"
				width={220}
				onClose={closeRoleMenu}
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
	.apikeys-panel {
		display: flex;
		flex-direction: column;
		height: 100%;
		overflow: hidden;
		width: 100%;
	}
	.name {
		color: var(--gray-900);
	}
	.prefix {
		font-family: var(--font-mono, monospace);
		font-size: var(--fs-xs);
		color: var(--gray-800);
	}
	.text-muted {
		color: var(--gray-800);
	}
	.error-container {
		padding: var(--space-sm);
	}
	.keys-table-wrap {
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
	:global(.keys-table-wrap tr:hover) .action-btn-wrap {
		visibility: visible;
	}
</style>
