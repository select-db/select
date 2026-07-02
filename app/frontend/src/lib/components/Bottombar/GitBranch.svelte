<script lang="ts">
	import Button from '$lib/system/Button/Button.svelte';
	import { gitFileStatusStore } from '$lib/components/views/Git/gitStore';
	import { modalStore } from '$lib/system/Modal/ModalStore';
	import GitBranchModal from './GitBranchModal.svelte';

	const status = $derived($gitFileStatusStore);
	const branchName = $derived(status?.branch || null);

	const openBranchSwitcher = () => {
		modalStore.set({
			content: () => GitBranchModal,
			width: 400
		});
	};
</script>

{#if branchName}
	<Button content={branchName} onclick={openBranchSwitcher} noRadius noBounce truncate />
{/if}
