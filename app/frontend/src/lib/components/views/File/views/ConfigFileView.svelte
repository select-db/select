<script lang="ts">
	import type { Tab } from '$lib/components/Layout/layoutStore';
	import { GetDefaultConfigContent } from '$lib/wailsjs/go/system/System';
	import { must, tryCatch } from '$lib/utils/tryCatch';
	import BaseFileView from './BaseFileView.svelte';
	import ConfigFileHeader from './ConfigFileHeader.svelte';

	type Props = {
		tab: Tab;
	};

	let { tab }: Props = $props();

	let defaultContent = $state<string | undefined>(undefined);
	$effect(() => {
		(async () => {
			defaultContent = await must(tryCatch(GetDefaultConfigContent));
		})();
	});
</script>

<BaseFileView {tab} language="json" manualSave {defaultContent}>
	{#snippet header({ hasUnsavedChanges, isModifiedFromDefault, saveFile })}
		<ConfigFileHeader {hasUnsavedChanges} {isModifiedFromDefault} onSave={saveFile} />
	{/snippet}
</BaseFileView>
