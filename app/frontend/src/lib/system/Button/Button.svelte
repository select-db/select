<script lang="ts">
	import Icon from '$lib/system/Icon/Icon.svelte';
	import Loader from '$lib/system/Loader/Loader.svelte';
	import Tooltip from '$lib/system/Tooltip/Tooltip.svelte';
	import type { Icons } from '$lib/system/Icon/types';
	import { tryCatch } from '$lib/utils/tryCatch';

	export type ButtonEmphasis = 'high' | 'low' | 'warning';

	type ButtonProps = {
		id?: string;
		leftIcon?: Icons;
		rightIcon?: Icons;
		iconSize?: number;
		content?: string;
		ariaLabel?: string;
		disabled?: boolean;
		active?: boolean;
		emphasis?: ButtonEmphasis;
		size?: 'sm' | 'md';
		badge?: string | number;
		style?: string;
		classes?: string;
		label?: string;

		noRadius?: boolean;
		noBounce?: boolean;
		noLoader?: boolean;
		noCapitalizeTooltip?: boolean;
		truncate?: boolean;
		/** When set, overrides internal loading state (e.g. when parent triggers action via keyboard). */
		loading?: boolean;

		onclick?: (e: MouseEvent) => void | Promise<void>;
		onmousedown?: (e: MouseEvent) => void | Promise<void>;
	};

	const {
		id,
		leftIcon,
		rightIcon,
		iconSize = 22,
		content,
		ariaLabel = content,
		disabled,
		active = false,
		emphasis = 'low',
		size = 'md',
		badge,
		style,
		classes,
		label,

		noRadius,
		noBounce,
		noLoader,
		noCapitalizeTooltip,
		truncate = false,
		loading: loadingProp,

		onclick,
		onmousedown
	}: ButtonProps = $props();

	let loadingInternal: boolean = $state(false);
	const loading = $derived(loadingProp !== undefined ? loadingProp : loadingInternal);

	const handleClick = async (e: MouseEvent) => {
		if (!onclick) return;
		if (loading || disabled) return;

		loadingInternal = !noLoader ? true : false;
		e.preventDefault();
		e.stopPropagation();

		await tryCatch(onclick, e);
		loadingInternal = false;
	};

	const handleMouseDown = async (e: MouseEvent) => {
		if (!onmousedown) return;
		if (loading || disabled) return;

		loadingInternal = !noLoader ? true : false;
		e.preventDefault();
		e.stopPropagation();

		await tryCatch(onmousedown, e);
		loadingInternal = false;
	};
</script>

{#snippet buttonContent()}
	<button
		{id}
		class="button {emphasis} {size} {classes}"
		class:disabled
		class:active
		class:noRadius
		class:noBounce
		class:loading
		class:truncate
		onclick={handleClick}
		onmousedown={handleMouseDown}
		aria-label={ariaLabel}
		{style}
	>
		{#if leftIcon}
			<Icon icon={leftIcon} size={iconSize} />
		{/if}
		{#if content}
			<p class="truncate">{content}</p>
		{/if}
		{#if rightIcon}
			<Icon icon={rightIcon} size={iconSize} />
		{/if}
		{#if badge}
			<p class="badge">{badge}</p>
		{/if}

		{#if loading && !noLoader}
			<div class="loading-overlay">
				<Loader size={iconSize} />
			</div>
		{/if}
	</button>
{/snippet}

{#if label}
	<Tooltip text={label} capitalize={!noCapitalizeTooltip}>
		{@render buttonContent()}
	</Tooltip>
{:else}
	{@render buttonContent()}
{/if}

<style>
	.button {
		display: flex;
		align-items: center;
		overflow: hidden;

		width: fit-content;
		min-height: 26px;

		transition:
			background-color 0.1s ease-out 0.03s,
			transform 0.05s ease;
	}

	.button:not(.noRadius) {
		border-radius: var(--br-xs);
	}

	.button.md {
		padding: var(--space-xs) var(--space-sm);
	}
	.button.sm {
		padding: var(--space-xs) var(--space-xs);
	}
	.button.xs {
		padding: 0 var(--space-xxs);
	}

	.button:active:not(.noBounce) {
		transform: scale(0.98);
	}
	.button p {
		padding: var(--space-xs);
		transition: color 0.1s ease-out 0.05s;
	}
	:global(.button.active svg) {
		stroke: var(--gray-1000);
	}
	.button.active p {
		color: var(--gray-1000);
	}
	/* @css-ignore */
	:global(.button svg) {
		transition: stroke 0.1s ease-out 0.05s;
	}

	.button.high {
		background-color: var(--gray-550);
		border: none;
	}
	.button.high p {
		color: var(--gray-900);
	}
	/* @css-ignore */
	:global(.button.high svg) {
		stroke: var(--gray-900);
	}
	.button.high:hover:not(.active) {
		background-color: var(--gray-0);
	}
	.button.high:hover p {
		color: var(--gray-1000);
	}
	:global(.button.high:hover svg) {
		stroke: var(--gray-1000);
	}

	.button.high.disabled,
	.button.high.disabled:hover {
		background-color: var(--gray-0);
		color: var(--gray-900);
		stroke: var(--gray-900);
	}
	:global(.button.low.disabled:hover svg) {
		stroke: var(--gray-900);
	}

	.button .badge {
		background-color: var(--gray-600);
		padding: var(--space-xxs) var(--space-xs);
		color: var(--gray-800);
		border-radius: var(--br-sm);
		height: fit-content;
		font-size: var(--fs-xs);
		margin: 0 var(--space-xs);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.button.low {
		background-color: transparent;
		border: none;
	}
	.button.low p {
		color: var(--gray-800);
	}
	:global(.button.low svg) {
		stroke: var(--gray-800);
	}
	.button.low:hover:not(.active) {
		background-color: var(--gray-550);
	}
	.button.low:hover p {
		color: var(--gray-1000);
	}
	:global(.button.low:hover svg) {
		stroke: var(--gray-1000);
	}

	.button.low.disabled,
	.button.low.disabled:hover {
		background-color: transparent;
		color: var(--gray-700);
		stroke: var(--gray-700);
	}

	:global(.button.low.disabled svg),
	:global(.button.low.disabled:hover svg) {
		stroke: var(--gray-700);
	}

	.button.warning {
		background-color: var(--orange-dark);
		border: none;
		font-weight: var(--fw-md);
	}
	.button.warning p {
		color: var(--gray-0);
	}
	:global(.button.warning svg) {
		stroke: var(--gray-0);
	}
	.button.warning:hover:not(.active):not(.disabled) {
		background-color: var(--orange);
	}
	.button.warning:hover:not(.disabled) p {
		color: var(--gray-0);
	}
	:global(.button.warning:hover:not(.disabled) svg) {
		stroke: var(--gray-0);
	}
	.button.warning.disabled,
	.button.warning.disabled:hover {
		background-color: var(--gray-550);
	}
	.button.warning.disabled p {
		color: var(--gray-700);
	}
	:global(.button.warning.disabled svg),
	:global(.button.warning.disabled:hover svg) {
		stroke: var(--gray-700);
	}

	/* Loading state */
	.button {
		position: relative;
	}

	.button.loading {
		pointer-events: none;
	}

	.loading-overlay {
		position: absolute;
		top: 0;
		left: 0;
		right: 0;
		bottom: 0;
		display: flex;
		align-items: center;
		justify-content: center;
		background-color: var(--gray-0);
		opacity: 0.7;
		border-radius: inherit;
	}
</style>
