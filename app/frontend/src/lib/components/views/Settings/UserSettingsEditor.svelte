<script lang="ts">
	import type { Tab } from '$lib/components/Layout/layoutStore';
	import type { graph } from '$lib/wailsjs/go/models';
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
			} as unknown as graph.FileNode
		}
	} as Tab);
</script>

<div class="user-settings-editor">
	{#if kind === 'theme'}
		<ThemeFileView {tab} />
	{:else}
		<ConfigFileView {tab} />
	{/if}
</div>

<style>
	.user-settings-editor {
		flex: 1;
		min-height: 0;
		display: flex;
		flex-direction: column;
	}
</style>
