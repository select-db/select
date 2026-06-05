<script lang="ts">
	import { updateTab, type Tab } from '$lib/components/Layout/layoutStore';

	import { throttle } from '$lib/utils/throttle';

	const MIN_HEIGHT = 0;

	type ResizerProps = {
		height: number;
		resizing?: boolean;
		tab: Tab;
	};
	let { height = $bindable(0), resizing = $bindable(false), tab }: ResizerProps = $props();

	let startY: number;
	let startHeight: number;

	const throttledResizeSidebar = throttle((e: MouseEvent) => {
		const deltaY = startY - e.clientY;
		height = Math.max(MIN_HEIGHT, startHeight + deltaY);
	}, 15);

	function startResizing(e: MouseEvent) {
		resizing = true;
		startY = e.clientY;
		startHeight = height;
		e.preventDefault();
		document.addEventListener('mousemove', throttledResizeSidebar);
		document.addEventListener('mouseup', stopResizing);
	}

	function stopResizing() {
		resizing = false;
		document.removeEventListener('mousemove', throttledResizeSidebar);
		document.removeEventListener('mouseup', stopResizing);

		if (!tab.file) return;

		updateTab({
			...tab,
			file: {
				...tab.file,
				tableHeight: height
			}
		});
	}
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="resizer" class:active={resizing} onmousedown={startResizing}></div>

<style>
	.resizer {
		position: absolute;
		left: 0;
		right: 0;

		height: 6px;
		border-top: var(--bw) transparent solid;

		cursor: row-resize;

		transition: border-top 0.2s 0.2s ease-in;
	}

	.resizer:hover,
	.resizer.active {
		border-top: var(--bw) var(--gray-700) solid;
	}

	.resizer:active {
		cursor: row-resize;
	}
</style>
