<script lang="ts">
	import type { Tab } from '$lib/components/Layout/layoutStore';
	import { GetDefaultUserConfigContent } from '$lib/wailsjs/go/system/System';
	import { must, tryCatch } from '$lib/utils/tryCatch';
	import BaseFileView from './BaseFileView.svelte';
	import ConfigFileHeader from './ConfigFileHeader.svelte';

	type Props = {
		tab: Tab;
		standalone?: boolean;
	};

	let { tab, standalone = false }: Props = $props();

	// .config is a personal file (keybindings/snippets) living in the per-user
	// config dir, addressed by a selectdb://user/... URI.
	let defaultContent = $state<string | undefined>(undefined);
	$effect(() => {
		(async () => {
			defaultContent = await must(tryCatch(GetDefaultUserConfigContent));
		})();
	});
</script>

<BaseFileView {tab} language="json" manualSave {defaultContent} {standalone}>
	{#snippet header({ hasUnsavedChanges, isModifiedFromDefault, saveFile })}
		<ConfigFileHeader {hasUnsavedChanges} {isModifiedFromDefault} onSave={saveFile} />
	{/snippet}
</BaseFileView>
