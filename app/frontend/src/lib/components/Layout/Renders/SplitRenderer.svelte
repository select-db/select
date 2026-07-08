<script lang="ts">
	import type { SplitContainer } from '../layoutStore';
	import Layout from '../Layout.svelte';
	import { layoutStore } from '../layoutStore';

	type Props = {
		split: SplitContainer;
	};

	let { split }: Props = $props();

	const children = $derived(split.children.filter((c): c is NonNullable<typeof c> => c != null));
	const isVertical = $derived(split.orientation === 'vertical');
	const flexDirection = $derived(isVertical ? 'row' : 'column');
	const childStyles = $derived(
		children.map(
			(_, i) =>
				`flex: 0 0 calc((100% - ${children.length - 1} * (2 * var(--space-xs) + var(--bw))) * ${split.sizes[i]}); overflow: hidden;`
		)
	);

	let containerElement: HTMLElement | null = $state(null);

	const MIN_SIZE_PX = 75; // Minimum size in pixels for each pane

	const clamp = (value: number, min: number, max: number) => Math.max(min, Math.min(max, value));

	const updateSplitSizes = (normalizedSizes: number[]) => {
		layoutStore.update((layout) => {
			const updateSplit = (node: typeof layout.root): typeof node => {
				if ('type' in node && node.type === 'split') {
					if (node.id === split.id) return { ...node, sizes: normalizedSizes };
					return { ...node, children: node.children.map(updateSplit) };
				}
				return node;
			};
			return { ...layout, root: updateSplit(layout.root) };
		});
	};

	const handleResizerMouseDown = (index: number) => (e: MouseEvent) => {
		e.preventDefault();
		if (!containerElement) return;

		const startPos = isVertical ? e.clientX : e.clientY;
		const startSizes = [...split.sizes];
		const containerRect = containerElement.getBoundingClientRect();
		const totalSize = isVertical ? containerRect.width : containerRect.height;

		// Convert pixel minimum to ratio based on container size
		const minRatio = MIN_SIZE_PX / totalSize;

		const handleMouseMove = (moveEvent: MouseEvent) => {
			const currentPos = isVertical ? moveEvent.clientX : moveEvent.clientY;
			const deltaRatio = (currentPos - startPos) / totalSize;

			const newSizes = [...startSizes];
			newSizes[index] = clamp(startSizes[index] + deltaRatio, minRatio, 1 - minRatio);
			newSizes[index + 1] = clamp(startSizes[index + 1] - deltaRatio, minRatio, 1 - minRatio);

			const sum = newSizes.reduce((acc, size) => acc + size, 0);
			const normalizedSizes = newSizes.map((size) => size / sum);

			updateSplitSizes(normalizedSizes);
		};

		const handleMouseUp = () => {
			document.removeEventListener('mousemove', handleMouseMove);
			document.removeEventListener('mouseup', handleMouseUp);
		};

		document.addEventListener('mousemove', handleMouseMove);
		document.addEventListener('mouseup', handleMouseUp);
	};
</script>

<div class="container" style="flex-direction: {flexDirection}" bind:this={containerElement}>
	{#each children as child, i (child.id)}
		<div style={childStyles[i]}>
			<Layout node={child} />
		</div>
		{#if i < children.length - 1}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div
				class="resizer"
				class:vertical={isVertical}
				class:horizontal={!isVertical}
				onmousedown={handleResizerMouseDown(i)}
			></div>
		{/if}
	{/each}
</div>

<style>
	.container {
		display: flex;
		height: 100%;
		gap: var(--space-xs);
	}

	.resizer {
		background-color: transparent;
		flex-shrink: 0;
		position: relative;
		z-index: 10;
		transition: border-color 0.15s ease-in-out;
	}

	.resizer.vertical {
		width: 4px;
		margin-right: -4px;
		cursor: col-resize;
		border-left: var(--bw) transparent solid;
		background-clip: padding-box;
	}

	.resizer.horizontal {
		height: 4px;
		margin-bottom: -4px;
		cursor: row-resize;
		border-top: var(--bw) transparent solid;
		background-clip: padding-box;
	}

	.resizer:hover,
	.resizer:active {
		border-color: var(--white-glow);
	}
</style>
