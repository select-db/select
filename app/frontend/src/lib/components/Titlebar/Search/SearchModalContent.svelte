<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import ResourceMenu from '$lib/components/ResourceMenu/ResourceMenu.svelte';
	import SearchScopePanel from './SearchScopePanel.svelte';
	import { parseDbInstanceIdFromSchemaId } from '$lib/components/ResourceMenu/resourceMenuScope';
	import { addTab, focusTab } from '$lib/components/Layout/layoutStore';
	import { quickActions, executeQuickAction } from '$lib/components/QuickActions/quickActionsData';
	import { modalStore } from '$lib/system/Modal/ModalStore';
	import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
	import { loadSchema } from '$lib/utils/query/loadSchema';
	import type { ResourceMenuOption, ResourceSearchScope } from '$lib/components/ResourceMenu/types';
	import { loadUserConfigResourceOptions } from '$lib/components/ResourceMenu/userConfigResources';
	import type { graph } from '$lib/wailsjs/go/models';
	import type { Component } from 'svelte';
	import ItemInfoModal from '$lib/components/views/FileSystem/modals/ItemInfoModal.svelte';
	import { readWorkspaceSearch, writeWorkspaceSearch } from './workspaceSearchStorage';

	const PANEL_MAX_HEIGHT = 400;

	type Props = {
		onClose: () => void;
	};

	let { onClose }: Props = $props();

	const databases = $derived($workspaceGraphStore?.db_instances ?? []);

	const persisted = readWorkspaceSearch();
	let searchQuery = $state('');
	let dbOn = $state<Record<string, boolean>>(persisted.dbOn);
	let schemaOn = $state<Record<string, boolean>>(persisted.schemaOn);

	// Personal config files (.theme, .config) live outside the workspace graph, so
	// they are surfaced here as extra options that open in the regular editor.
	let userConfigOptions = $state<ResourceMenuOption[]>([]);

	function syncKeyMap(prev: Record<string, boolean>, keys: string[]): Record<string, boolean> {
		const next: Record<string, boolean> = {};
		for (const id of keys) {
			next[id] = id in prev ? prev[id]! : true;
		}
		return next;
	}

	$effect(() => {
		const dbs = databases;
		const dbIds = dbs.map((d) => d.id);
		const schemaIds = dbs.flatMap((db) =>
			(db.children ?? []).filter((c) => c.type === 'schema').map((c) => c.id)
		);
		untrack(() => {
			dbOn = syncKeyMap(dbOn, dbIds);
			schemaOn = syncKeyMap(schemaOn, schemaIds);
		});
	});

	$effect(() => {
		writeWorkspaceSearch({ query: '', dbOn, schemaOn });
	});

	const searchScope = $derived.by((): ResourceSearchScope | undefined => {
		const dbs = databases;
		if (dbs.length === 0) return undefined;

		const knownSchemaIds = new Set<string>();
		for (const db of dbs) {
			for (const ch of db.children ?? []) {
				if (ch.type === 'schema') knownSchemaIds.add(ch.id);
			}
		}

		const enabledDbIds = new Set<string>();
		for (const db of dbs) {
			if (dbOn[db.id] !== false) enabledDbIds.add(db.id);
		}

		const enabledSchemaIds = new Set<string>();
		for (const sid of knownSchemaIds) {
			if (schemaOn[sid] === false) continue;
			const dbId = parseDbInstanceIdFromSchemaId(sid);
			if (dbId && dbOn[dbId] !== false) enabledSchemaIds.add(sid);
		}

		return { knownSchemaIds, enabledDbIds, enabledSchemaIds };
	});

	onMount(() => {
		for (const db of databases) {
			if ((db.children?.length ?? 0) !== 0) continue;
			loadSchema({ database: db, silent: true });
		}
		loadUserConfigResourceOptions().then((opts) => {
			userConfigOptions = opts;
		});
	});

	function handleSelect(option: ResourceMenuOption) {
		if (option.type === 'quick_action') {
			executeQuickAction(option);
			onClose();
		} else if (['settings', 'schema', 'chat', 'terminal', 'diff'].includes(option.type)) {
			focusTab(option.uri);
			onClose();
		} else if (option.type === 'file' || option.type === 'temp_file') {
			addTab(option.node as graph.FileNode);
			onClose();
		} else if (option.type === 'db_instance') {
			addTab(option.node as graph.DBInstanceNode);
			onClose();
		} else if (option.type === 'db_item') {
			modalStore.set({
				content: () => ItemInfoModal as unknown as Component,
				props: { item: option.node },
				width: 600
			});
		}
	}
</script>

<div class="search-modal" style:max-height="{PANEL_MAX_HEIGHT}px">
	<SearchScopePanel {databases} bind:dbOn bind:schemaOn maxHeight={PANEL_MAX_HEIGHT} />
	<div class="results-panel">
		<ResourceMenu
			types={[
				'file',
				'temp_file',
				'db_instance',
				'db_item',
				'quick_action',
				'settings',
				'schema',
				'chat',
				'terminal',
				'diff'
			]}
			bind:searchQuery
			action={handleSelect}
			placeholder="Search workspace..."
			width={470}
			maxHeight={PANEL_MAX_HEIGHT}
			noBorder
			extraOptions={[...quickActions, ...userConfigOptions]}
			{searchScope}
		/>
	</div>
</div>

<style>
	.search-modal {
		display: flex;
		align-items: stretch;
		min-height: 0;
	}

	.results-panel {
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
		min-height: 0;
	}
</style>
