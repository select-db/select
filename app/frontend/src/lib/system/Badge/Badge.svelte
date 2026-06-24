<script lang="ts">
	import Icon from '$lib/system/Icon/Icon.svelte';

	let {
		status,
		label = false,
		size = 12,
		class: className = ''
	}: {
		status: 'success' | 'error';
		label?: string | boolean;
		size?: number;
		class?: string;
	} = $props();

	const ICON = { success: 'check', error: 'cross' } as const;
	const DEFAULT_LABEL = { success: 'Success', error: 'Error' } as const;

	const text = $derived(
		typeof label === 'string' ? label : label ? DEFAULT_LABEL[status] : ''
	);
</script>

<span class="badge {status} {className}" class:has-label={text}>
	<Icon icon={ICON[status]} {size} stroke="var(--badge-fg)" strokeWidth={2} />
	{#if text}<p class="label">{text}</p>{/if}
</span>

<style>
	.badge {
		display: inline-flex;
		align-items: center;
		gap: var(--space-xxs);
		padding: var(--space-xxs);
		border-radius: var(--br-xs);
		line-height: 1;
		color: var(--badge-fg);
		background: var(--badge-bg);
	}

	.badge > p {
		padding: var(--space-xxs);
	}

	.badge.has-label {
		padding: var(--space-xxs) var(--space-xs);
		font-size: var(--fs-sm);
	}

	.badge.success {
		--badge-fg: var(--green-dark);
		--badge-bg: color-mix(in srgb, var(--green) 14%, transparent);
	}

	.badge.error {
		--badge-fg: var(--red);
		--badge-bg: color-mix(in srgb, var(--red) 14%, transparent);
	}
</style>
