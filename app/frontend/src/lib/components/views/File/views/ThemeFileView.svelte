<script lang="ts">
	import type { Tab } from '$lib/components/Layout/layoutStore';
	import { GetDefaultThemeContent } from '$lib/wailsjs/go/system/System';
	import { must, tryCatch } from '$lib/utils/tryCatch';
	import BaseFileView from './BaseFileView.svelte';
	import ThemeFileHeader from './ThemeFileHeader.svelte';

	type Props = {
		tab: Tab;
		standalone?: boolean;
	};

	let { tab, standalone = false }: Props = $props();

	let defaultContent = $state<string | undefined>(undefined);
	$effect(() => {
		(async () => {
			defaultContent = await must(tryCatch(GetDefaultThemeContent));
		})();
	});
</script>

<BaseFileView {tab} language="css" manualSave {defaultContent} {standalone}>
	{#snippet header({ hasUnsavedChanges, isModifiedFromDefault, saveFile })}
		<ThemeFileHeader {hasUnsavedChanges} {isModifiedFromDefault} onSave={saveFile} />
	{/snippet}
</BaseFileView>
