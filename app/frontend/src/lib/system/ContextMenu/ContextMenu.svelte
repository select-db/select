<script lang="ts">
	import type { ContextMenu } from '$lib/system/ContextMenu/types';
	import Portal from '$lib/system/Portal/Portal.svelte';
	import FloatingBox from '$lib/system/FloatingBox/FloatingBox.svelte';
	import Menu from '$lib/system/Menu/Menu.svelte';
	import type { MenuOption } from '$lib/system/Menu/Menu.types';

	let {
		position,
		options,
		direction,
		onClose,
		metadata,
		anchor,
		menuWidth = 175
	}: {
		position: ContextMenu['position'];
		options: ContextMenu['options'];
		direction: ContextMenu['direction'];
		onClose: () => void;
		metadata: ContextMenu['metadata'];
		anchor: 'cursor' | 'child';
		menuWidth?: number;
	} = $props();

	// Convert ContextMenuOptions to MenuOptions
	const menuOptions = $derived(
		options.map(
			(opt, index): MenuOption => ({
				id: index.toString(),
				label: opt.label,
				icon: opt.icon ?? undefined,
				type: opt.divider ? 'divider' : 'option',
				disabled: false,
				action: opt.action
					? async () => {
							await opt.action?.(onClose, metadata);
						}
					: undefined
			})
		)
	);

	// When anchoring to the cursor, apply an offset to position the menu
	const OFFSET = $derived(anchor === 'cursor' ? (direction === 'left' ? 20 : -20) : 0);
</script>

<Portal>
	<FloatingBox {position} offset={{ x: OFFSET, y: OFFSET }} onBackdropClick={onClose} backdrop>
		<Menu
			options={menuOptions}
			{onClose}
			maxHeight={400}
			highlightActiveOption={true}
			width={menuWidth}
		/>
	</FloatingBox>
</Portal>
