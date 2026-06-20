<script lang="ts">
	import Button from '$lib/system/Button/Button.svelte';
	import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
	import { modalStore } from '$lib/system/Modal/ModalStore';
	import WorkspaceMenuContent from './WorkspaceMenuContent.svelte';

	const graph = $derived($workspaceGraphStore);
	const workspaceName = $derived(graph?.name ?? 'Workspace');

	const openWorkspaceMenu = () => {
		modalStore.set({
			content: () => WorkspaceMenuContent,
			width: 360
		});
	};
</script>

<Button content={workspaceName} onclick={openWorkspaceMenu} noBounce truncate size="sm" />
