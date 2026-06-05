<script lang="ts">
	import { leftPanelTab } from '$lib/components/Leftbar/store';
	import FileSystem from '$lib/components/views/FileSystem/FileSystem.svelte';
	import GitPanel from '$lib/components/views/Git/GitPanel.svelte';
</script>

{#if $leftPanelTab === 'files'}
	<FileSystem />
{:else if $leftPanelTab === 'github-branch'}
	<GitPanel />
{/if}
