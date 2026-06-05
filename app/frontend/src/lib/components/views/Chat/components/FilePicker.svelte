<script lang="ts">
	import Button from '$lib/system/Button/Button.svelte';
	import Portal from '$lib/system/Portal/Portal.svelte';
	import FloatingBox from '$lib/system/FloatingBox/FloatingBox.svelte';
	import ResourceMenu from '$lib/components/ResourceMenu/ResourceMenu.svelte';
	import type { ResourceMenuOption } from '$lib/components/ResourceMenu/types';

	type Props = {
		selectedIds: string[];
		onToggle: (option: ResourceMenuOption) => void;
	};

	let { selectedIds, onToggle }: Props = $props();

	let showPicker = $state(false);
	let anchorEl = $state<HTMLElement | null>(null);

	function open() {
		showPicker = true;
	}

	function close() {
		showPicker = false;
	}
</script>

<div bind:this={anchorEl}>
	<Button emphasis="low" size="sm" leftIcon="paperclip" iconSize={18} onclick={open} />
</div>

{#if showPicker}
	<Portal>
		<FloatingBox anchor={anchorEl} offset={{ x: 0, y: 8 }} backdrop onBackdropClick={close}>
			<ResourceMenu
				types={['file', 'temp_file']}
				action={onToggle}
				multiple
				{selectedIds}
				placeholder="Search files..."
				width={400}
				maxHeight={350}
				onClose={close}
			/>
		</FloatingBox>
	</Portal>
{/if}
