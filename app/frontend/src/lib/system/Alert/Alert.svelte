<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { AlertType } from '$lib/system/Alert/types';
	import Button from '$lib/system/Button/Button.svelte';
	import { notify } from '$lib/system/Notifications/notificationsStore';

	let {
		type,
		message,
		copyable,
		noPulse = false,
		onClose,
		style
	}: {
		type: AlertType;
		message: string;
		copyable?: boolean;
		noPulse?: boolean;
		onClose?: () => void;
		style?: string;
	} = $props();

	const colorMap: Record<typeof type, [number, number, number]> = {
		default: [0, 180, 255], // cyan
		success: [0, 200, 100], // green
		error: [255, 60, 60] // red
	};

	let borderColor = $state('');
	let rafId: number;
	let startTime = 0;

	const interpolateColor = (
		from: [number, number, number],
		to: [number, number, number],
		ratio: number
	) => {
		const r = Math.round(from[0] + (to[0] - from[0]) * ratio);
		const g = Math.round(from[1] + (to[1] - from[1]) * ratio);
		const b = Math.round(from[2] + (to[2] - from[2]) * ratio);
		return `rgb(${r}, ${g}, ${b})`;
	};

	function animateBorder() {
		const base = colorMap[type] ?? colorMap.default;
		const variant = base.map((c) => Math.min(255, c + 40)) as [number, number, number];
		const duration = 5000;

		const loop = (timestamp: number) => {
			if (!startTime) startTime = timestamp;
			const elapsed = (timestamp - startTime) % duration;
			const ratio = 0.5 + 0.5 * Math.sin((elapsed / duration) * Math.PI * 2);

			borderColor = interpolateColor(base, variant, ratio);
			rafId = requestAnimationFrame(loop);
		};

		rafId = requestAnimationFrame(loop);
	}

	onMount(() => {
		animateBorder();
	});

	onDestroy(() => {
		cancelAnimationFrame(rafId);
	});

	const copy = () => {
		navigator.clipboard.writeText(message);
		notify({
			type: AlertType.Success,
			message: 'Copied to clipboard'
		});
	};
</script>

<div class="alert {type}" class:noPulse style="--glow-color: {borderColor}; {style}">
	<pre class="message selectable">{message}</pre>
	<div class="actions">
		{#if copyable}
			<Button onclick={copy} emphasis="low" leftIcon="copy" iconSize={16} size="sm" />
		{/if}
		{#if onClose}
			<Button onclick={onClose} emphasis="low" leftIcon="cross" iconSize={16} size="sm" />
		{/if}
	</div>
</div>

<style>
	.alert {
		display: flex;
		align-items: start;
		padding: var(--space-xs);
		border: 0.5px solid var(--glow-color);
		border-radius: var(--br-xs);
		background-color: var(--gray-200);
		transition:
			border-color 0.3s ease,
			background-color 0.2s ease;
		width: fit-content;
		margin: 0 auto;
	}

	.alert:not(.noPulse) {
		animation: pulse 1.5s infinite ease-in-out;
	}

	.message {
		margin: var(--space-xs) 0 0 0;
		padding: var(--space-xxs) var(--space-xs) var(--space-xs) var(--space-xs);
		font-weight: 400;
		font-size: var(--fs-sm);
		flex: 1;
		white-space: pre-wrap;
		word-break: break-all;
	}

	.actions {
		display: flex;
		gap: var(--space-xs);
		align-items: center;
	}

	@keyframes pulse {
		0%,
		50% {
			transform: scale(1.005);
		}
	}
</style>
