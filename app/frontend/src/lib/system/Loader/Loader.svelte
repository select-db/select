<script lang="ts">
	type LoaderProps = {
		size?: number;
	};
	let { size = 18 }: LoaderProps = $props();
</script>

<div class="loader" style={`width: ${size}px`}></div>

<style>
	.loader {
		width: 50px;
		--b: 4px;
		aspect-ratio: 1;
		border-radius: 50%;
		padding: 1px;
		background: conic-gradient(#0000 10%, var(--gray-1000)) content-box;
		-webkit-mask:
			repeating-conic-gradient(#0000 0deg, #000 1deg 20deg, #0000 21deg 36deg),
			radial-gradient(farthest-side, #0000 calc(100% - var(--b) - 1px), #000 calc(100% - var(--b)));
		mask:
			repeating-conic-gradient(#0000 0deg, #000 1deg 20deg, #0000 21deg 36deg),
			radial-gradient(farthest-side, #0000 calc(100% - var(--b) - 1px), #000 calc(100% - var(--b)));
		-webkit-mask-composite: destination-in;
		mask-composite: intersect;
		animation: l4 1s infinite steps(10);
	}
	@keyframes l4 {
		to {
			transform: rotate(1turn);
		}
	}
</style>
