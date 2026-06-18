<script lang="ts">
	import Button from '$lib/system/Button/Button.svelte';
	import { modalStore } from '$lib/system/Modal/ModalStore';
	import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
	import SearchModalContent from './SearchModalContent.svelte';
	import type { Component } from 'svelte';

	const workspaceName = $derived($workspaceGraphStore?.name ?? 'Workspace');
</script>

<div class="search">
	<Button
		content={workspaceName}
		leftIcon="search"
		iconSize={15}
		onclick={() =>
			modalStore.set({
				content: () => SearchModalContent as unknown as Component,
				width: 650
			})}
		size="sm"
	/>
</div>
