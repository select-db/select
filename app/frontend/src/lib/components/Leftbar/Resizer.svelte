<script lang="ts">
	import { updateIsLeftbarOpened } from '$lib/components/Leftbar/store';
	import { throttle } from '$lib/utils/throttle';

	const MIN_WIDTH = 0;
	const DEFAULT_WIDTH = 200;

	export let width: number = parseInt(
		localStorage.getItem('sidebarWidth') || `${DEFAULT_WIDTH}`,
		10
	);
	export let resizing: boolean = false;

	const throttledResizeSidebar = throttle((e: MouseEvent) => {
		width = Math.max(MIN_WIDTH, Math.min(e.clientX - 4, window.innerWidth / 2));
		updateIsLeftbarOpened(width > MIN_WIDTH);
		localStorage.setItem('sidebarWidth', width.toString());
	}, 15);

	function startResizing(e: MouseEvent) {
		resizing = true;
		e.preventDefault();
		document.addEventListener('mousemove', throttledResizeSidebar);
		document.addEventListener('mouseup', stopResizing);
	}

	function stopResizing() {
		resizing = false;
		document.removeEventListener('mousemove', throttledResizeSidebar);
		document.removeEventListener('mouseup', stopResizing);
	}
</script>

<div
	id="resizer"
	role="presentation"
	class={resizing ? 'resizing' : ''}
	onmousedown={startResizing}
></div>

<style>
	#resizer {
		position: absolute;
		z-index: 3;
		top: 48px;
		right: 1px;
		bottom: var(--space-sm-md);

		width: 10px;
		border-right: var(--bw) transparent solid;
		margin-right: -1px;

		cursor: col-resize;

		transition: border-right 0.2s 0.2s ease-in;
	}

	#resizer:hover,
	#resizer.resizing {
		border-right: var(--bw) var(--white-glow) solid;
	}

	#resizer:active {
		cursor: col-resize;
	}
</style>
