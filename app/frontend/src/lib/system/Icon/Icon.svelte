<script lang="ts">
	import { icons, type Icons } from './types';

	let {
		icon,
		size = 24,
		stroke = 'var(--gray-800)',
		fill = 'none',
		strokeWidth = 1.5,
		class: className = ''
	}: {
		icon: Icons;
		size?: number;
		stroke?: string;
		fill?: string;
		strokeWidth?: number;
		class?: string;
	} = $props();
</script>

{#if icons[icon]}
	<span class="icon-shell {className}">
		<svg
			xmlns="http://www.w3.org/2000/svg"
			width={size}
			height={size}
			style:min-width="{size}px"
			viewBox={icons[icon].viewBox}
			{fill}
			{stroke}
			stroke-width={strokeWidth}
			stroke-linejoin="round"
			shape-rendering="geometricPrecision"
			preserveAspectRatio="xMidYMid meet"
		>
			<!-- eslint-disable-next-line svelte/no-at-html-tags intended -->
			{@html icons[icon].svg}
		</svg>
	</span>
{/if}

<style>
	.icon-shell {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		line-height: 0;
		transform: translateZ(
			0
		); /* Creates compositing layer to prevent pixel-shifting during repaints */
	}

	.icon-shell svg {
		display: block; /* Removes inline baseline alignment that causes sub-pixel shifts */
	}
</style>
