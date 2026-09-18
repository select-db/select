<script lang="ts">
	import Flag from './Flag.svelte';
	import { REGIONS, type Region } from './regions';

	let {
		current = '',
		onSelect
	}: {
		/** Currently selected server domain. */
		current?: string;
		onSelect: (domain: string) => void;
	} = $props();

	const isSelected = (r: Region) => r.available && r.domain === current;
</script>

<div class="regions" role="radiogroup" aria-label="Region">
	{#each REGIONS as r (r.id)}
		<button
			type="button"
			class="region"
			class:selected={isSelected(r)}
			role="radio"
			aria-checked={isSelected(r)}
			disabled={!r.available}
			onclick={() => r.domain && onSelect(r.domain)}
		>
			<span class="flag"><Flag code={r.flag} /></span>
			<span class="text">
				<span class="label">{r.label}</span>
				<span class="location">{r.location}</span>
			</span>
			{#if r.available}
				<span class="dot" aria-hidden="true"></span>
			{:else}
				<span class="badge">Coming soon</span>
			{/if}
		</button>
	{/each}
</div>

<style>
	.regions {
		display: flex;
		flex-direction: column;
		gap: var(--space-sm);
		width: 280px;
	}

	.region {
		display: flex;
		align-items: center;
		gap: var(--space-sm);
		padding: var(--space-sm) var(--space-md);
		background: var(--gray-0);
		border: var(--border);
		border-radius: var(--br-sm);
		color: var(--gray-900);
		text-align: left;
		cursor: pointer;
		transition: border-color 0.12s ease;
	}

	.region:hover:not(:disabled) {
		background: var(--gray-100);
	}

	.region.selected {
		border-color: var(--gray-900);
	}

	.region:disabled {
		cursor: default;
		color: var(--gray-700);
	}

	.flag {
		display: inline-flex;
		flex-shrink: 0;
		line-height: 0;
		border-radius: 2px;
		overflow: hidden;
		box-shadow: inset 0 0 0 0.5px var(--shadow);
	}

	.region:disabled .flag {
		opacity: 0.5;
	}

	.text {
		display: flex;
		flex-direction: column;
		flex: 1;
		min-width: 0;
		line-height: 1.2;
	}

	.label {
		font-size: var(--fs-sm);
		font-weight: 500;
	}

	.location {
		font-size: var(--fs-xs);
		color: var(--gray-700);
	}

	.dot {
		width: 9px;
		height: 9px;
		flex-shrink: 0;
		border: 1.5px solid var(--gray-400);
		border-radius: 50%;
	}

	.region.selected .dot {
		border-color: var(--gray-900);
		background: radial-gradient(circle, var(--gray-900) 45%, transparent 46%);
	}

	.badge {
		flex-shrink: 0;
		font-size: var(--fs-xs);
		color: var(--gray-700);
		background: var(--gray-100);
		border: var(--border);
		border-radius: var(--br-sm);
		padding: var(--space-xs) var(--space-sm);
		white-space: nowrap;
	}
</style>
