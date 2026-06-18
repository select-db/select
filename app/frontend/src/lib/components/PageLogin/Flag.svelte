<script lang="ts">
	import type { FlagCode } from './regions';

	let { code, size = 18 }: { code: FlagCode; size?: number } = $props();

	const W = 24;
	const H = 18;
	const stripeH = H / 13;

	// US: 7 red stripes + a canton with a small grid of stars.
	const cantonW = W * 0.42;
	const cantonH = stripeH * 7;
	const usStars: { x: number; y: number }[] = [];
	for (let r = 0; r < 4; r++) {
		for (let c = 0; c < 5; c++) {
			const x = (c + (r % 2 ? 1 : 0.5)) * (cantonW / 5.5);
			if (x < cantonW - 1.2) usStars.push({ x, y: (r + 0.7) * (cantonH / 4) });
		}
	}

	// EU: ring of 12 stars on a blue field.
	const euStars = Array.from({ length: 12 }, (_, i) => {
		const a = (i / 12) * Math.PI * 2 - Math.PI / 2;
		return { x: W / 2 + Math.cos(a) * H * 0.31, y: H / 2 + Math.sin(a) * H * 0.31 };
	});
</script>

<svg
	width={size * (W / H)}
	height={size}
	viewBox="0 0 {W} {H}"
	role="img"
	aria-label={code === 'us' ? 'United States' : 'European Union'}
>
	{#if code === 'us'}
		<rect width={W} height={H} fill="#fff" />
		{#each [0, 2, 4, 6, 8, 10, 12] as y (y)}
			<rect y={y * stripeH} width={W} height={stripeH} fill="#b22234" />
		{/each}
		<rect width={cantonW} height={cantonH} fill="#3c3b6e" />
		{#each usStars as s, i (i)}
			<circle cx={s.x} cy={s.y} r="0.5" fill="#fff" />
		{/each}
	{:else}
		<rect width={W} height={H} fill="#003399" />
		{#each euStars as s, i (i)}
			<circle cx={s.x} cy={s.y} r="0.9" fill="#ffcc00" />
		{/each}
	{/if}
</svg>
