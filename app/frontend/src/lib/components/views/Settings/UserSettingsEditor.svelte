<script lang="ts">
	import { onMount } from 'svelte';
	import {
		getActiveTab,
		updateSettingsTab,
		type Tab
	} from '$lib/components/Layout/layoutStore';
	import type * as graph from '$lib/wails/graph';
	import { EnsureUserConfigDefaults } from '$lib/bindings/selectDb/internal/system/system';
	import { tryCatch } from '$lib/utils/tryCatch';
	import ThemeFileView from '$lib/components/views/File/views/ThemeFileView.svelte';
	import ConfigFileView from '$lib/components/views/File/views/ConfigFileView.svelte';

	// Personal .theme / .config live in the per-user config dir (outside any
	// workspace) and are edited through their selectdb://user/... URIs. We reuse
	// the regular file editor views (same Monaco editor + Apply/Reset header) by
	// wrapping each file in a synthetic, layout-detached Tab.
	type Props = {
		kind: 'theme' | 'config';
	};

	let { kind }: Props = $props();

	const fileName = $derived(kind === 'theme' ? '.theme' : '.config');
	const uri = $derived(`selectdb://user/${fileName}`);

	// We snapshot the last-saved view state on init (to restore after the remount) 
	// and push updates back into the settings tab as it moves.
	const savedViewState = getActiveTab()?.settings?.editors?.[kind]?.viewState;

	function persistViewState(viewState: unknown) {
		const editors = getActiveTab()?.settings?.editors;
		updateSettingsTab({ editors: { ...editors, [kind]: { viewState } } });
	}

	// The editor reads the file directly by URI, so make sure it exists on disk
	// (seeded from defaults) before we render the view; otherwise a missing file
	// would fail to open.
	let ready = $state(false);
	onMount(async () => {
		await tryCatch(EnsureUserConfigDefaults);
		ready = true;
	});

	const tab = $derived({
		id: `user-${kind}-settings`,
		uri,
		file: {
			node: {
				id: uri,
				name: fileName,
				type: 'file',
				path: '',
				uri,
				folder_id: '',
				badges: [],
				convertValues: () => ({})
			} as unknown as graph.FileNode,
			editor: { viewState: savedViewState }
		}
	} as Tab);
</script>

<div class="user-settings-editor">
	{#if ready && kind === 'theme'}
		<ThemeFileView {tab} standalone onStateChange={persistViewState} />
	{:else if ready}
		<ConfigFileView {tab} standalone onStateChange={persistViewState} />
	{/if}
</div>

<style>
	.user-settings-editor {
		flex: 1;
		min-height: 0;
		display: flex;
		flex-direction: column;

		margin: var(--space-sm);
		overflow: hidden;
		border-radius: var(--br-sm);
		border: var(--border);
	}
</style>
