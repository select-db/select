<script lang="ts">
	import { core } from '$lib/wailsjs/go/models';
	import { SvelteSet } from 'svelte/reactivity';

	import Tooltip from '$lib/system/Tooltip/Tooltip.svelte';

	import type { graph } from '$lib/wailsjs/go/models';

	type Props = {
		result: graph.ExplainResult;
	};

	let { result }: Props = $props();

	const explain = $derived(result?.root || null);

	type FlatRow = {
		level: number;
		node: core.ExplainNode;
		isLast: boolean;
		branches: number[];
	};

	const flattenTree = (
		node: core.ExplainNode,
		level: number,
		isLast: boolean,
		branches: number[]
	): FlatRow[] => {
		const rows: FlatRow[] = [];
		rows.push({ level, node, isLast, branches: [...branches] });

		if (!isLast) branches.push(level);

		if (node.children && node.children.length > 0) {
			node.children.forEach((child, index) => {
				const childIsLast = index === node.children.length - 1;
				rows.push(...flattenTree(child, level + 1, childIsLast, branches));
			});
		}

		if (!isLast) branches.pop();

		return rows;
	};

	const flatRows = $derived.by(() => {
		if (!explain) return [];
		return flattenTree(explain, 0, true, []);
	});

	const getMax = (getter: (r: FlatRow) => number) => {
		if (!explain) return 0;
		return Math.max(...flatRows.map(getter), 0);
	};

	const maxCost = $derived.by(() =>
		getMax((r) => r.node.exclusiveCost ?? r.node.estimatedCost ?? 0)
	);
	const maxRows = $derived.by(() => getMax((r) => r.node.actualRows || r.node.estimatedRows || 0));
	const maxTime = $derived.by(() => getMax((r) => r.node.exclusiveTime || r.node.actualTime || 0));
	const totalExecutionTime = $derived.by(() => explain?.actualTime || 0);
	const maxSharedHit = $derived.by(() => getMax((r) => r.node.cacheStats?.sharedHitBlocks || 0));
	const maxSharedRead = $derived.by(() => getMax((r) => r.node.cacheStats?.sharedReadBlocks || 0));
	const maxTempRead = $derived.by(() => getMax((r) => r.node.cacheStats?.tempReadBlocks || 0));
	const maxTempWrite = $derived.by(() => getMax((r) => r.node.cacheStats?.tempWriteBlocks || 0));

	const formatValue = (v: number | null | undefined) =>
		v?.toLocaleString('en-US', { maximumFractionDigits: 2 }) || '0';
	const formatBlocks = (v: number | null | undefined) =>
		v && v > 0 ? v.toLocaleString('en-US', { maximumFractionDigits: 0 }) : '0';
	const formatBlocksAsBytes = (v: number | null | undefined, f?: string | null) =>
		f || formatBlocks(v);
	const formatPercent = (v: number | null | undefined, t: number) =>
		v && t > 0 ? (v / t) * 100 : 0;

	let expandedRows = new SvelteSet<string>();
	const toggleRow = (id: string) => {
		if (expandedRows.has(id)) expandedRows.delete(id);
		else expandedRows.add(id);
	};
</script>

{#if explain}
	<div class="explain-grid scrollable">
		<table class="grid-table">
			<thead>
				<tr>
					<th class="node-index sticky" rowspan="2"> # </th>
					<th class="" rowspan="2">
						<Tooltip text="Actual execution time spent in this operation" position="top">
							Time
						</Tooltip>
					</th>
					<th class="node-type" rowspan="2">
						<Tooltip text="The database operation being performed" position="top">
							Operation
						</Tooltip>
					</th>
					<th class="text-end" rowspan="2">
						<Tooltip text="Number of rows processed by this operation" position="top">Rows</Tooltip>
					</th>
					<th class="" rowspan="2">
						<Tooltip text="Estimation accuracy: ratio of actual vs estimated rows" position="top">
							Est.
						</Tooltip>
					</th>
					<th class="text-end" rowspan="2">
						<Tooltip text="Query planner's estimated cost for this operation" position="top">
							Cost
						</Tooltip>
					</th>
					<th class="text-end" colspan="2">
						<Tooltip text="Shared buffer cache statistics" position="top">Shared</Tooltip>
					</th>
					<th class="text-end last" colspan="2">
						<Tooltip text="Temporary buffer statistics" position="top">Temp</Tooltip>
					</th>
				</tr>
				<tr>
					<th class="text-end label">
						<Tooltip text="Data blocks found in cache (faster)" position="top">Hit</Tooltip>
					</th>
					<th class="text-end label">
						<Tooltip text="Data blocks read from disk (slower)" position="top">Read</Tooltip>
					</th>
					<th class="text-end label">
						<Tooltip text="Temporary data blocks read from disk" position="top">Read</Tooltip>
					</th>
					<th class="text-end label last">
						<Tooltip text="Temporary data blocks written to disk" position="top">Write</Tooltip>
					</th>
				</tr>
			</thead>
			<tbody>
				{#each flatRows as row (row.node.id)}
					{@const node = row.node}
					{@const isExpanded = expandedRows.has(node.id)}
					{@const displayTime = node.exclusiveTime ?? node.actualTime}

					<tr
						class="row-wrapper"
						class:never-executed={!node.actualTime && !node.estimatedCost}
						onclick={() => toggleRow(node.id)}
					>
						<td class="node-index sticky">{node.id.split('_')[1] || node.id}</td>

						{@render ProgressCell({
							value: displayTime,
							max: maxTime,
							format: (v) => `${formatValue(v)}ms`,
							detail:
								isExpanded && totalExecutionTime > 0
									? `${formatPercent(displayTime, totalExecutionTime).toFixed(1)}%`
									: null
						})}

						<td class="node-type">
							{@render LevelDivider({
								level: row.level,
								isLast: row.isLast,
								branches: row.branches
							})}
							<span>{node.operation}</span>
							<!-- eslint-disable-next-line svelte/require-each-key -->
							{#each [{ label: 'on', value: node.targetTable }, { label: 'using', value: node.indexName }, { label: 'on', value: node.condition }, { label: 'by', value: node.sortKey?.join(', ') }] as detail}
								{#if detail.value}
									<span class="node-detail"
										>{detail.label} <span class="node-value">{detail.value}</span></span
									>
								{/if}
							{/each}
							{#if isExpanded}
								<div class="node-details">
									{#if node.warnings && node.warnings.length > 0}
										<div class="warnings">
											<!-- eslint-disable-next-line svelte/require-each-key -->
											{#each node.warnings as warning}
												<div class="warning">{warning}</div>
											{/each}
										</div>
									{/if}
									{#if node.metadata && Object.keys(node.metadata).length > 0}
										<table class="metadata-table">
											<tbody>
												<!-- eslint-disable-next-line svelte/require-each-key -->
												{#each Object.entries(node.metadata) as [key, value]}
													<tr class="metadata-row">
														<td class="metadata-key">{key}:</td>
														<td class="metadata-value">{String(value)}</td>
													</tr>
												{/each}
											</tbody>
										</table>
									{/if}
								</div>
							{/if}
						</td>

						{@render ProgressCell({
							value: node.actualRows ?? node.estimatedRows,
							max: maxRows,
							format: formatValue,
							align: 'end'
						})}

						<td class="text-end grid-progress-cell">
							{#if node.estimatedRows != null && node.actualRows != null}
								{@const factor = node.actualRows / node.estimatedRows}
								{#if factor !== 1}
									{@const isWithin2x = factor >= 0.5 && factor <= 2}
									<div class="metric-value">
										<span class:good={isWithin2x} class:bad={!isWithin2x}>
											{factor.toFixed(2)}x
										</span>
									</div>
									{#if isExpanded && node.estimatedRows != null}
										<div class="metric-detail">
											{formatValue(node.estimatedRows)}
										</div>
									{/if}
								{:else}
									<span class="text-muted">-</span>
								{/if}
							{:else}
								<span class="text-muted">-</span>
							{/if}
						</td>

						{@render ProgressCell({
							value: node.exclusiveCost ?? node.estimatedCost,
							max: maxCost,
							format: formatValue,
							detail:
								isExpanded && node.estimatedCost
									? `${formatPercent(node.exclusiveCost ?? node.estimatedCost, maxCost).toFixed(1)}%`
									: null,
							align: 'end'
						})}

						{@render CacheStatCell({
							value: node.cacheStats?.sharedHitBlocks,
							max: maxSharedHit,
							formatted: node.cacheStats?.sharedHitBlocksFormatted,
							isExpanded
						})}
						{@render CacheStatCell({
							value: node.cacheStats?.sharedReadBlocks,
							max: maxSharedRead,
							formatted: node.cacheStats?.sharedReadBlocksFormatted,
							isExpanded
						})}
						{@render CacheStatCell({
							value: node.cacheStats?.tempReadBlocks,
							max: maxTempRead,
							formatted: node.cacheStats?.tempReadBlocksFormatted,
							isExpanded
						})}
						{@render CacheStatCell({
							value: node.cacheStats?.tempWriteBlocks,
							max: maxTempWrite,
							formatted: node.cacheStats?.tempWriteBlocksFormatted,
							isExpanded
						})}
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{/if}

{#snippet ProgressCell({
	value,
	max,
	format,
	detail,
	align
}: {
	value: number | null | undefined;
	max: number;
	format: (v: number) => string;
	detail?: string | null;
	align?: 'start' | 'end';
})}
	<td class="text-end grid-progress-cell">
		{#if value != null && max > 0}
			<div class="progress-bar">
				<div class="progress-fill" style="--progress-width: {formatPercent(value, max)}%"></div>
			</div>
			<div class="metric-value {align === 'end' ? 'text-end' : ''}">{format(value)}</div>
			{#if detail}
				<div class="metric-detail {align === 'end' ? 'text-end' : ''}">
					<span class="text-muted">{detail}</span>
				</div>
			{/if}
		{:else}
			<span class="text-muted">-</span>
		{/if}
	</td>
{/snippet}

{#snippet CacheStatCell({
	value,
	max,
	formatted,
	isExpanded
}: {
	value: number | null | undefined;
	max: number;
	formatted?: string | null;
	isExpanded: boolean;
})}
	<td class="text-end grid-progress-cell">
		{#if value && max > 0}
			<div class="progress-bar">
				<div class="progress-fill" style="--progress-width: {formatPercent(value, max)}%"></div>
			</div>
			<div class="metric-value text-end">{formatBlocks(value)}</div>
			{#if isExpanded}
				<div class="metric-detail text-end">{formatBlocksAsBytes(value, formatted)}</div>
			{/if}
		{:else}
			<span class="text-muted text-end">-</span>
		{/if}
	</td>
{/snippet}

{#snippet LevelDivider({
	level,
	isLast,
	branches
}: {
	level: number;
	isLast: boolean;
	branches: number[];
})}
	<div class="node-operation">
		<!-- eslint-disable-next-line svelte/require-each-key -->
		{#each Array.from({ length: level }, (_, i) => i) as i}
			<div class="branch-line" class:show={branches.includes(i)}></div>
		{/each}
		{#if level > 0}
			<div class="branch-connector" class:last={isLast}></div>
		{/if}
	</div>
{/snippet}

<style>
	.explain-grid {
		height: 100%;
		overflow: auto;
		background-color: var(--gray-100);
	}

	.grid-table {
		width: 100%;
		border-collapse: collapse;
		font-size: 0.875rem;
		min-width: 100%;
		border: none;
	}

	.grid-table thead {
		position: sticky;
		top: 0;
		z-index: 10;
	}

	.grid-table th {
		font-size: var(--fs-sm);
		position: sticky;
		top: 0;
		z-index: 1;
		padding: var(--space-xs) var(--space-sm);
		text-align: left;
		font-weight: var(--fw-light);
		color: var(--gray-800);
		border-left: none;
		border-right: none;
	}

	.grid-table thead tr:last-child th {
		padding-top: var(--space-xxs);
		padding-bottom: var(--space-xs);
		z-index: 1;
	}

	.grid-table th.last {
		padding-right: var(--space-md) !important;
	}

	.grid-table thead tr:first-child th.sticky {
		z-index: 6 !important;
		top: 0 !important;
		position: sticky !important;
		left: 0 !important;
	}

	.grid-table th.text-end {
		text-align: right;
	}

	th.label {
		font-size: var(--fs-xs) !important;
	}

	.grid-table tr:first-child th {
		border-top: none;
	}

	.grid-table tr:first-child th,
	.grid-table tr:last-child th {
		border-bottom: none;
	}

	.grid-table td {
		font-size: var(--fs-sm);
		padding: var(--space-sm) var(--space-sm) calc(var(--space-xs-sm) + 2px) var(--space-sm);
		text-align: left;
		vertical-align: center;
		white-space: nowrap;

		border-left: none;
		border-right: none;
	}

	.grid-table tbody tr td:last-child {
		padding-right: var(--space-md);
	}

	.grid-table tr:last-child td {
		border-bottom: var(--border);
	}

	.row-wrapper {
		transition: background-color 0.15s ease;
	}

	.row-wrapper:hover td {
		background-color: var(--gray-0);
	}

	.row-wrapper:hover .metric-value {
		color: var(--gray-1000);
	}

	.row-wrapper.never-executed {
		opacity: 0.6;
	}

	.node-index {
		min-width: 32px !important;
		text-align: end !important;
		font-size: var(--fs-sm);
		font-family: monospace;
		color: var(--gray-800);
		padding-left: var(--space-sm-md) !important;
		padding-right: var(--space-sm-md) !important;
	}

	/* Sticky index column */
	th.sticky {
		position: sticky;
		left: 0;
		top: 0;
		z-index: 5 !important;
		background: var(--gray-200);
		box-shadow: 0.5px 0 0 0 var(--shadow);
	}

	td.sticky {
		position: sticky;
		left: 0;
		z-index: 2;
		background: var(--gray-100);
		box-shadow: 0.5px 0 0 0 var(--shadow);
	}

	.row-wrapper:hover td.sticky {
		background-color: var(--gray-0);
	}

	.text-end {
		text-align: right;
		white-space: nowrap;
	}

	.grid-progress-cell {
		position: relative;
		min-width: 80px;
	}

	/* Narrow cache columns - last 4 columns (Shared Hit, Shared Read, Temp Read, Temp Write) */
	.grid-table thead tr:first-child th:nth-child(n + 7),
	.grid-table thead tr:last-child th,
	.grid-table tbody td:nth-child(n + 8) {
		min-width: 60px;
		max-width: 75px;
		padding-left: var(--space-xs);
		padding-right: var(--space-xs);
	}

	.progress-bar {
		position: absolute;
		top: 0;
		left: 0;
		right: 0;
		height: 100%;
		opacity: 0.3;
	}

	.progress-fill {
		width: 0%;
		height: 100%;
		animation: progressFill 0.2s forwards;
		z-index: 1;

		background: var(--blue);
		border-radius: 0 var(--br-xs) var(--br-sm) 0;
	}

	@keyframes progressFill {
		from {
			width: 0%;
		}
		to {
			width: var(--progress-width, 0%);
		}
	}

	.metric-value {
		position: relative;
		z-index: 1;

		color: var(--gray-1000);
		font-family: monospace;
		font-size: var(--fs-sm);
	}

	.metric-detail {
		font-size: var(--fs-sm);
		color: var(--gray-800);
		margin-top: var(--space-xs);
	}

	.text-muted {
		color: var(--gray-800);
	}

	.node-type {
		width: 100%;
		position: relative;
		white-space: normal;
	}

	.node-operation {
		display: inline-flex;
		align-items: center;
		margin-right: var(--space-xs);
		vertical-align: middle;
	}

	.branch-line {
		margin-top: -4px;
		width: 5px;
		height: 22px;
		margin-right: 16px;
	}

	.branch-line.show {
		border-right: 1px solid var(--gray-800);
	}

	.branch-connector {
		height: 14px;
		width: 8px;
		margin-left: 4px;
		margin-top: -12px;
		border-left: 1px solid var(--gray-800);
		border-right: none;
		border-bottom: 1px solid var(--gray-800);
	}

	.node-detail {
		margin-left: var(--space-xs);
		margin-right: var(--space-sm);
		color: var(--gray-800);
		font-size: var(--fs-sm);
	}

	.node-value {
		color: var(--gray-1000);
		font-family: monospace;
		margin-left: 2px;
	}

	.node-details {
		margin-top: var(--space-sm-md);
		padding-top: var(--space-sm);
		border-top: var(--border);
	}

	.warnings {
		margin-bottom: var(--space-sm);
	}

	.warning {
		font-size: var(--fs-sm);
		font-family: monospace;
		padding: var(--space-xs);
		padding-left: var(--space-sm);
		border-left: 2px solid var(--red-glow);
		border-radius: var(--br-xs);
		color: var(--red-glow);
		margin-bottom: var(--space-xs);
	}

	.metadata-table {
		margin-top: -8px;
		width: 100%;
		border-collapse: collapse;
		font-size: var(--fs-sm);
		border-bottom: none !important;
	}

	/* .metadata-table tbody tr:not(:last-child) {
		border-bottom: 1px solid var(--gray-300);
	}

	.metadata-table tbody tr:last-child {
		border-bottom: none;
	} */

	.metadata-key {
		padding: var(--space-xs) var(--space-sm);
		padding-right: var(--space-md);
		color: var(--gray-800);
		font-weight: var(--fw-light);
		white-space: nowrap;
		vertical-align: top;
	}

	.metadata-value {
		padding: var(--space-xs) var(--space-sm);
		color: var(--gray-1000);
		font-family: monospace;
		word-break: break-word;
		width: 100%;
	}

	.good {
		color: var(--green-glow);
	}

	.bad {
		color: var(--red-glow);
	}
</style>
