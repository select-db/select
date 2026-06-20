<script lang="ts">
	import type { Tab } from '$lib/components/Layout/layoutStore';
	import {
		GetDefaultConfigContent,
		GetDefaultUserConfigContent
	} from '$lib/wailsjs/go/system/System';
	import { must, tryCatch } from '$lib/utils/tryCatch';
	import BaseFileView from './BaseFileView.svelte';
	import ConfigFileHeader from './ConfigFileHeader.svelte';

	type Props = {
		tab: Tab;
	};

	let { tab }: Props = $props();

	// The personal .config (keybindings/snippets) lives in the per-user config dir
	// and is addressed by a selectdb://user/... URI; the workspace .config
	// (execution limits) lives at the workspace root. They have different defaults
	// and reset targets.
	const isUserConfig = $derived(tab.file?.node?.uri?.startsWith('selectdb://user/') ?? false);

	let defaultContent = $state<string | undefined>(undefined);
	$effect(() => {
		const userScoped = isUserConfig;
		(async () => {
			defaultContent = await must(
				tryCatch(userScoped ? GetDefaultUserConfigContent : GetDefaultConfigContent)
			);
		})();
	});
</script>

<BaseFileView {tab} language="json" manualSave {defaultContent}>
	{#snippet header({ hasUnsavedChanges, isModifiedFromDefault, saveFile })}
		<ConfigFileHeader {hasUnsavedChanges} {isModifiedFromDefault} {isUserConfig} onSave={saveFile} />
	{/snippet}
</BaseFileView>
