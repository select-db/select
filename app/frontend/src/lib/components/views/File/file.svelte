<script lang="ts">
	import type { Tab } from '$lib/components/Layout/layoutStore';

	import SqlFileView from './views/SqlFileView.svelte';
	import ThemeFileView from './views/ThemeFileView.svelte';
	import ConfigFileView from './views/ConfigFileView.svelte';
	import LintFileView from './views/LintFileView.svelte';
	import GenericFileView from './views/GenericFileView.svelte';

	type Props = {
		tab: Tab;
	};

	let { tab }: Props = $props();

	const file = $derived(tab.file?.node);
	const isSql = $derived(file?.name.endsWith('.sql') ?? false);
	const isTheme = $derived(file?.name === '.theme');
	const isConfig = $derived(file?.name === '.config');
	const isLint = $derived(file?.name === '.lint');
</script>

{#if isSql}
	<SqlFileView {tab} />
{:else if isTheme}
	<ThemeFileView {tab} />
{:else if isConfig}
	<ConfigFileView {tab} />
{:else if isLint}
	<LintFileView {tab} />
{:else}
	<GenericFileView {tab} />
{/if}
