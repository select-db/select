<script lang="ts">
	import type { Tab } from '$lib/components/Layout/layoutStore';
	import { getFileLanguage } from '../Editor/utils/getFileLanguage';
	import BaseFileView from './BaseFileView.svelte';

	type Props = {
		tab: Tab;
	};

	let { tab }: Props = $props();

	const file = $derived(tab.file?.node);
	const language = $derived(file ? getFileLanguage(file.name) : 'plaintext');
</script>

<BaseFileView {tab} {language} />
