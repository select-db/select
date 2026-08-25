<script lang="ts">
	import type { Tab } from '$lib/components/Layout/layoutStore';
	import { GetDefaultLintContent } from '$lib/bindings/selectDb/internal/system/system';
	import { must, tryCatch } from '$lib/utils/tryCatch';
	import BaseFileView from './BaseFileView.svelte';
	import LintFileHeader from './LintFileHeader.svelte';

	type Props = {
		tab: Tab;
	};

	let { tab }: Props = $props();

	let defaultContent = $state<string | undefined>(undefined);
	$effect(() => {
		(async () => {
			defaultContent = await must(tryCatch(GetDefaultLintContent));
		})();
	});
</script>

<BaseFileView {tab} language="json" manualSave {defaultContent}>
	{#snippet header({ hasUnsavedChanges, isModifiedFromDefault, saveFile })}
		<LintFileHeader {hasUnsavedChanges} {isModifiedFromDefault} onSave={saveFile} />
	{/snippet}
</BaseFileView>
