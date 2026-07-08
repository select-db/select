<script lang="ts" generics="T">
	import type { Snippet } from 'svelte';
	import Input from '$lib/system/Input/Input.svelte';

	type Column = {
		key: string;
		label?: string;
		width?: string;
		align?: 'left' | 'right';
		searchable?: boolean;
		pinned?: boolean;
	};

	let {
		columns,
		rows,
		getKey,
		filterValue,
		onRowClick,
		cell,
		empty
	}: {
		columns: Column[];
		rows: T[];
		getKey: (row: T) => string;
		filterValue?: (row: T) => string;
		onRowClick?: (row: T) => void;
		cell: Snippet<[string, T]>;
		empty?: Snippet;
	} = $props();

	let search = $state('');

	const filtered = $derived(
		search && filterValue
			? rows.filter((r) => filterValue(r).toLowerCase().includes(search.toLowerCase()))
			: rows
	);

	// cumulative left offsets for pinned columns, computed from declared widths
	const pinnedOffsets = $derived.by(() => {
		const offsets: Record<string, number> = {};
		let cumulative = 0;
		for (const col of columns) {
			if (col.pinned) {
				offsets[col.key] = cumulative;
				const w = parseFloat(col.width ?? '0');
				cumulative += isNaN(w) ? 0 : w;
			}
		}
		return offsets;
	});

	const lastPinnedKey = $derived([...columns].reverse().find((c) => c.pinned)?.key ?? null);
</script>

<div class="table-scroll scrollable">
	<table>
		<thead>
			<tr>
				{#each columns as col (col.key)}
					<th
						style:width={col.width}
						style:left={col.pinned ? `${pinnedOffsets[col.key]}px` : undefined}
						class:right={col.align === 'right'}
						class:th-search={col.searchable}
						class:pinned={col.pinned}
						class:last-pinned={col.key === lastPinnedKey}
					>
						{#if col.searchable}
							<Input bind:value={search} placeholder="Search…" clearable emphasis="low" size="md" />
						{:else if col.label}
							<span class="col-label">{col.label}</span>
						{/if}
					</th>
				{/each}
				<th class="filler"></th>
			</tr>
		</thead>
		<tbody>
			{#each filtered as row (getKey(row))}
				<tr class:clickable={!!onRowClick} onclick={() => onRowClick?.(row)}>
					{#each columns as col (col.key)}
						<td
							style:left={col.pinned ? `${pinnedOffsets[col.key]}px` : undefined}
							class="truncate"
							class:right={col.align === 'right'}
							class:pinned={col.pinned}
							class:last-pinned={col.key === lastPinnedKey}
						>
							{@render cell(col.key, row)}
						</td>
					{/each}
					<td class="filler"></td>
				</tr>
			{:else}
				<tr class="empty-row">
					<td colspan={columns.length + 1}>
						{#if empty}
							{@render empty()}
						{:else}
							<span class="empty-text">{search ? 'No results.' : 'No record to display.'}</span>
						{/if}
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>

<style>
	.table-scroll {
		display: flex;
		overflow: auto;
		min-height: 0;
		border: var(--border);
		border-radius: var(--br-sm);
	}

	table {
		flex: 1;
		table-layout: fixed;
		border-collapse: separate;
		border-spacing: 0;
		font-size: var(--fs-sm);
	}

	thead th {
		position: sticky;
		top: 0;
		background: var(--gray-200);
		border-bottom: var(--border);
		height: 32px;
		padding: 0 var(--space-sm);
		text-align: left;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		z-index: 1;
	}

	thead th span {
		font-size: var(--fs-xs);
		color: var(--gray-800);
	}

	thead th.pinned {
		position: sticky;
		z-index: 3;
	}

	th.last-pinned,
	td.last-pinned {
		border-right: var(--border);
	}

	thead th.th-search {
		padding: var(--space-xs);
		height: auto;
	}

	thead th.right {
		text-align: right;
	}

	tbody tr.clickable {
		cursor: pointer;
	}

	tbody tr.clickable:hover td {
		background-color: var(--gray-100);
	}

	td {
		padding: 0 var(--space-sm);
		height: 36px;
		vertical-align: middle;
		color: var(--gray-900);
		background: var(--gray-200);
		border-bottom: var(--border);
		overflow: hidden;
	}

	tbody tr:last-child td {
		border-bottom: none;
	}

	td.pinned {
		position: sticky;
		z-index: 1;
	}

	tbody tr.clickable:hover td.pinned {
		background: var(--gray-100);
	}

	td.right {
		text-align: right;
	}

	.empty-row td {
		padding: 0 var(--space-sm-md);
		color: var(--gray-800);
	}

	.filler {
		width: auto;
	}
</style>
