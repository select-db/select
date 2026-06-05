<script lang="ts">
	import { onMount, onDestroy } from 'svelte';

	export let error = false;

	const START_PROGRESS = 5;
	const END_PROGRESS = 100;
	const DURATION_MS = 90_000; // 1.5 min, matches the auth client's overall budget

	let progress = START_PROGRESS;
	let color = '';
	let startTime = 0;
	let rafId: number;

	type RGB = [number, number, number];

	const COLOR_STEPS: RGB[] = [
		[0, 180, 255], // blue
		[100, 220, 255], // light cyan
		[255, 160, 0], // orange
		[255, 60, 60] // red
	];

	function rgb(rgb: RGB) {
		return `rgb(${rgb.join(',')})`;
	}

	function interpolateColor(from: RGB, to: RGB, ratio: number): string {
		const r = Math.round(from[0] + (to[0] - from[0]) * ratio);
		const g = Math.round(from[1] + (to[1] - from[1]) * ratio);
		const b = Math.round(from[2] + (to[2] - from[2]) * ratio);
		return `rgb(${r}, ${g}, ${b})`;
	}

	function interpolateMultiStepColor(steps: RGB[], t: number): string {
		const n = steps.length - 1;
		const step = t * n;
		const i = Math.floor(step);
		const ratio = step - i;

		if (i >= n) return rgb(steps[n]);

		const from = steps[i];
		const to = steps[i + 1];
		return interpolateColor(from, to, ratio);
	}

	function animate(timestamp: number) {
		if (!startTime) startTime = timestamp;

		const elapsed = timestamp - startTime;
		const ratio = Math.min(elapsed / DURATION_MS, 1);

		progress = START_PROGRESS + (END_PROGRESS - START_PROGRESS) * ratio;
		color = interpolateMultiStepColor(COLOR_STEPS, ratio);

		if (!error && ratio < 1) {
			rafId = requestAnimationFrame(animate);
		}
	}

	function forceErrorState() {
		cancelAnimationFrame(rafId);
		progress = 100;
		color = 'rgb(255, 60, 60)';
	}

	let timeLeft = Math.floor(DURATION_MS / 1000);
	let interval: ReturnType<typeof setInterval>;

	function formatTime(seconds: number) {
		const mins = Math.floor(seconds / 60)
			.toString()
			.padStart(2, '0');
		const secs = Math.floor(seconds % 60)
			.toString()
			.padStart(2, '0');
		return `${mins}:${secs}`;
	}

	onMount(() => {
		if (error) {
			forceErrorState();
		} else {
			startTime = 0;
			rafId = requestAnimationFrame(animate);

			interval = setInterval(() => {
				if (timeLeft > 0) {
					timeLeft -= 1;
				} else {
					clearInterval(interval);
				}
			}, 1000);
		}
	});

	onDestroy(() => {
		cancelAnimationFrame(rafId);
		clearInterval(interval);
	});

	$: if (error) {
		forceErrorState();
		clearInterval(interval);
		timeLeft = 0;
	}
</script>

<div class="progress-container">
	<div
		class="progress-fill"
		style="width: {progress}%; background-color: {color}; box-shadow: 0 0 4px {color}, 0 0 8px {color};"
	>
		<div
			class="glow-head"
			style="background: {color}; box-shadow:
				0 0 8px {color},
				0 0 16px {color},
				0 0 24px {color};"
		></div>
	</div>
</div>

{#if !error}
	<div class="timer" style="border-color: {color}; --glow-color: {color}">
		{formatTime(timeLeft)}
	</div>
{/if}

<style>
	.progress-container {
		width: 100%;
		height: 2px;
	}

	.progress-fill {
		display: flex;
		justify-content: end;
		height: 100%;
		transition: width 0.2s ease;
	}

	.glow-head {
		width: 20px;
		height: 2px;
	}

	.timer {
		text-align: center;
		font-size: 1rem;
		font-family: monospace;
		padding: var(--space-sm) var(--space-md);
		border-radius: var(--br-sm);
		border: 1px solid;
		width: fit-content;
		margin: var(--space-md) auto 0;
		color: var(--gray-900);
		box-shadow: 0 0 5px var(--glow-color);
		transition:
			color 0.3s,
			border-color 0.3s;
	}
</style>
