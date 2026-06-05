<script lang="ts">
	import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
	import Select from '$lib/system/Select/Select.svelte';
	import type { SelectOption } from '$lib/system/Select/Select.types';
	import DatabaseIndicator from '$lib/components/shared/DatabaseIndicator/DatabaseIndicator.svelte';
	import DatabaseGroupIndicator from '$lib/components/shared/DatabaseIndicator/DatabaseGroupIndicator.svelte';

	type Props = {
		value?: string | string[];
		multiple?: boolean;
		onchange?: (databaseId: string | string[]) => void;
		open?: boolean;
	};

	let { value, multiple = false, onchange, open = $bindable(false) }: Props = $props();

	const options = $derived.by(() => {
		const graph = $workspaceGraphStore;
		if (!graph) return [];
		return graph.db_instances.map((db) => ({
			label: db.name,
			value: db.id
		}));
	});

	const internalValue = $derived.by(() =>
		multiple ? (Array.isArray(value) ? value : value ? [value] : []) : (value ?? '')
	);
</script>

<div class="picker">
	<Select
		{multiple}
		value={internalValue}
		options={options}
		placeholder="Select a db..."
		searchEnabled={true}
		searchPlaceholder="Search db..."
		onchange={(v) => onchange?.(v as string | string[])}
		menuWidth={275}
		size="xs"
		emphasis="low"
		bind:open
	>
		{#snippet optionDisplay(option: SelectOption<string> | null)}
			{#if option}
				<span class="db-option">
					<DatabaseIndicator id={option.value} size={17} loaderSize={15} />
					<span class="db-option-label">{option.label}</span>
				</span>
			{:else}
				<span class="db-option-placeholder">Select a db...</span>
			{/if}
		{/snippet}

		{#snippet summaryDisplay(selected: SelectOption<string>[])}
			<span class="db-option">
				<DatabaseGroupIndicator ids={selected.map((s) => s.value)} size={17} loaderSize={15} />
				<span class="db-option-label">{selected.length} databases</span>
			</span>
		{/snippet}
	</Select>
</div>

<style>
	.picker {
		margin-bottom: -1px;
	}

	.db-option {
		display: flex;
		align-items: center;
		gap: var(--space-sm);
		min-width: 0;
		width: 100%;
	}

	.db-option-label {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.db-option-placeholder {
		color: var(--gray-800);
	}
</style>
