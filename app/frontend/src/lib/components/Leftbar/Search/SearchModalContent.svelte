<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import ResourceMenu from '$lib/components/ResourceMenu/ResourceMenu.svelte';
	import SearchScopePanel from './SearchScopePanel.svelte';
	import { parseDatasourceIdFromSchemaId } from '$lib/components/ResourceMenu/resourceMenuScope';
	import { addTab, focusTab } from '$lib/components/Layout/layoutStore';
	import {
		quickActions,
		settingsQuickActions,
		executeQuickAction
	} from '$lib/components/QuickActions/quickActionsData';
	import { modalStore } from '$lib/system/Modal/ModalStore';
	import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
	import { loadSchemaIfEmpty } from '$lib/utils/query/loadSchema';
	import type { ResourceMenuOption, ResourceSearchScope } from '$lib/components/ResourceMenu/types';
	import { GetFileNodeByID } from '$lib/wails/graph';
	import type * as graph from '$lib/wails/graph';
	import type { Component } from 'svelte';
	import ItemInfoModal from '$lib/components/views/FileSystem/modals/ItemInfoModal.svelte';
	import { readWorkspaceSearch, writeWorkspaceSearch } from './workspaceSearchStorage';

	const PANEL_MAX_HEIGHT = 400;

	type Props = {
		onClose: () => void;
	};

	let { onClose }: Props = $props();

	const datasources = $derived($workspaceGraphStore?.datasources ?? []);

	const persisted = readWorkspaceSearch();
	let searchQuery = $state('');
	let datasourceOn = $state<Record<string, boolean>>(persisted.datasourceOn);
	let schemaOn = $state<Record<string, boolean>>(persisted.schemaOn);

	function syncKeyMap(prev: Record<string, boolean>, keys: string[]): Record<string, boolean> {
		const next: Record<string, boolean> = {};
		for (const id of keys) {
			next[id] = id in prev ? prev[id]! : true;
		}
		return next;
	}

	$effect(() => {
		const datasourceIds = datasources.map((d) => d.id);
		const schemaIds = datasources.flatMap((db) =>
			db.children.filter((c) => c.type === 'schema').map((c) => c.id)
		);
		untrack(() => {
			datasourceOn = syncKeyMap(datasourceOn, datasourceIds);
			schemaOn = syncKeyMap(schemaOn, schemaIds);
		});
	});

	$effect(() => {
		writeWorkspaceSearch({ query: '', datasourceOn, schemaOn });
	});

	const searchScope = $derived.by((): ResourceSearchScope | undefined => {
		if (datasources.length === 0) return undefined;

		// eslint-disable-next-line svelte/prefer-svelte-reactivity -- local set built and consumed within this derivation
		const knownSchemaIds = new Set<string>();
		for (const db of datasources) {
			for (const ch of db.children) {
				if (ch.type === 'schema') knownSchemaIds.add(ch.id);
			}
		}

		// eslint-disable-next-line svelte/prefer-svelte-reactivity -- local set built and consumed within this derivation
		const enabledDatasourceIds = new Set<string>();
		for (const db of datasources) {
			if (datasourceOn[db.id] !== false) enabledDatasourceIds.add(db.id);
		}

		// eslint-disable-next-line svelte/prefer-svelte-reactivity -- local set built and consumed within this derivation
		const enabledSchemaIds = new Set<string>();
		for (const sid of knownSchemaIds) {
			if (schemaOn[sid] === false) continue;
			const datasourceId = parseDatasourceIdFromSchemaId(sid);
			if (datasourceId && datasourceOn[datasourceId] !== false) enabledSchemaIds.add(sid);
		}

		return { knownSchemaIds, enabledDatasourceIds, enabledSchemaIds };
	});

	onMount(() => {
		for (const db of datasources) void loadSchemaIfEmpty(db);
	});

	async function handleSelect(option: ResourceMenuOption) {
		if (option.type === 'quick_action') {
			executeQuickAction(option);
			onClose();
		} else if (['settings', 'schema', 'chat', 'terminal', 'diff'].includes(option.type)) {
			focusTab(option.uri);
			onClose();
		} else if (option.type === 'file') {
			// A row built from a recently opened file carries only what the
			// recents list remembers, so the file is fetched before it is opened
			// — otherwise the tab would come up without its databases.
			const node = (await GetFileNodeByID(option.uri)) ?? (option.node as graph.FileNode);
			addTab(node);
			onClose();
		} else if (option.type === 'temp_file') {
			addTab(option.node as graph.FileNode);
			onClose();
		} else if (option.type === 'datasource') {
			addTab(option.node as graph.DatasourceNode);
			onClose();
		} else if (option.type === 'datasource_item') {
			modalStore.set({
				content: () => ItemInfoModal as unknown as Component,
				props: { item: option.node },
				width: 600
			});
		}
	}
</script>

<div class="search-modal" style:max-height="{PANEL_MAX_HEIGHT}px">
	<SearchScopePanel {datasources} bind:datasourceOn bind:schemaOn maxHeight={PANEL_MAX_HEIGHT} />
	<div class="results-panel">
		<ResourceMenu
			types={[
				'file',
				'temp_file',
				'datasource',
				'datasource_item',
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
			extraOptions={[...quickActions, ...settingsQuickActions]}
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
