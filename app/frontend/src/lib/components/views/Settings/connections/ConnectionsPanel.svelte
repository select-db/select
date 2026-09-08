<script lang="ts">
	import type { Component } from 'svelte';
	import Table from '$lib/system/Table/Table.svelte';
	import Button from '$lib/system/Button/Button.svelte';
	import Alert from '$lib/system/Alert/Alert.svelte';
	import { AlertType } from '$lib/system/Alert/types';
	import Icon from '$lib/system/Icon/Icon.svelte';
	import { modalStore } from '$lib/system/Modal/ModalStore';
	import { tryCatch } from '$lib/utils/tryCatch';
	import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
	import {
		ListDatasources,
		DeleteDatasource
	} from '$lib/bindings/selectDb/internal/datasource/datasource';
	import type * as datasource from '$lib/bindings/selectDb/internal/datasource/models';
	import type * as graph from '$lib/wails/graph';
	import RevokeConnectionModal from '$lib/components/views/Database/RevokeConnectionModal.svelte';

	/**
	 * The proxified connections this workspace has, whether or not anything in
	 * the workspace still points at them.
	 *
	 * That gap is the reason for the screen. A connection is named by a directory
	 * that git replicates, so it can be deleted on another machine, in a branch,
	 * or outside the app entirely, while the credential stays on the server. Once
	 * that happens there is nowhere else it can be seen: the id needed to name it
	 * lived in the file that was deleted.
	 */
	type Connection = datasource.ListedDatasource;

	const columns = [
		{ key: 'name', label: 'Name', searchable: true, width: '220px', pinned: true },
		{ key: 'db_type', label: 'Dialect', width: '120px' },
		{ key: 'referenced', label: 'In this workspace', width: '170px' },
		{ key: 'actions', label: '', width: '110px' }
	];

	let connections = $state<Connection[]>([]);
	let loadError = $state<string | null>(null);
	let loading = $state(true);

	/** Every proxified database id the workspace files still name. */
	const referencedIds = $derived.by(() => {
		const ids: string[] = [];
		const walk = (folder: { folders?: (graph.FolderNode | null)[]; db_instances?: unknown[] }) => {
			for (const db of (folder.db_instances ?? []) as (graph.DBInstanceNode | null)[]) {
				if (db?.proxified) ids.push(db.id);
			}
			for (const child of folder.folders ?? []) if (child) walk(child);
		};
		const g = $workspaceGraphStore;
		if (g) walk(g);
		return ids;
	});

	const unreferenced = $derived(connections.filter((c) => !referencedIds.includes(c.id)));

	const load = async () => {
		loading = true;
		const [rows, err] = await tryCatch(ListDatasources);
		loading = false;
		if (err) {
			loadError = err.message;
			return;
		}
		loadError = null;
		connections = rows ?? [];
	};

	$effect(() => {
		void load();
	});

	const revoke = (connection: Connection) => {
		modalStore.set({
			content: (() => RevokeConnectionModal) as () => Component,
			width: 520,
			props: {
				names: [connection.name],
				onCancel: () => modalStore.set(null),
				onConfirm: async () => {
					modalStore.set(null);
					const [, err] = await tryCatch(DeleteDatasource, connection.id);
					if (err) {
						loadError = err.message;
						return;
					}
					await load();
				}
			}
		});
	};
</script>

<div class="panel">
	<div class="header">
		<div>
			<p class="title">Connections</p>
			<p class="hint">
				Databases whose credentials are stored on the server rather than in this workspace. Revoking
				one drops those credentials for everyone.
			</p>
		</div>
	</div>

	{#if unreferenced.length > 0}
		<div class="notice">
			<Alert
				type={AlertType.Default}
				message={`${unreferenced.length} ${unreferenced.length === 1 ? 'connection is' : 'connections are'} not used by anything in this workspace. Their credentials are still stored and still work.`}
				noPulse
			/>
		</div>
	{/if}

	{#if loadError}
		<div class="notice">
			<Alert type={AlertType.Error} message={loadError} noPulse />
		</div>
	{:else if !loading && connections.length === 0}
		<p class="empty">No shared connections in this workspace.</p>
	{:else}
		<div class="table-wrap">
			<Table
				{columns}
				rows={connections}
				getKey={(c) => c.id}
				filterValue={(c) => c.name ?? ''}
				{cell}
			/>
		</div>
	{/if}
</div>

{#snippet cell(key: string, connection: Connection)}
	{#if key === 'name'}
		<span class="selectable truncate">{connection.name || connection.id}</span>
	{:else if key === 'db_type'}
		<span class="text-muted">{connection.db_type}</span>
	{:else if key === 'referenced'}
		{#if referencedIds.includes(connection.id)}
			<span class="text-muted">Yes</span>
		{:else}
			<span class="orphan">
				<Icon icon="info" size={12} />
				No
			</span>
		{/if}
	{:else if key === 'actions'}
		{#if connection.can_manage}
			<Button content="Revoke" emphasis="low" size="sm" onclick={() => revoke(connection)} />
		{/if}
	{/if}
{/snippet}

<style>
	.panel {
		display: flex;
		flex-direction: column;
		gap: var(--space-sm);
		height: 100%;
		min-height: 0;
	}

	.header {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		gap: var(--space-md);
	}

	.title {
		margin: 0;
		font-weight: 600;
		color: var(--gray-1000);
	}

	.hint,
	.empty {
		margin: var(--space-xxs) 0 0;
		color: var(--gray-700);
	}

	.notice {
		flex-shrink: 0;
	}

	.table-wrap {
		flex: 1;
		min-height: 0;
	}

	.orphan {
		display: inline-flex;
		align-items: center;
		gap: var(--space-xxs);
		color: var(--yellow);
	}
</style>
