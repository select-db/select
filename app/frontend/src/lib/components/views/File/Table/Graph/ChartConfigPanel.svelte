<script lang="ts">
	import type { GraphConfig } from '$lib/components/Layout/layoutStore';
	import type { Icons } from '$lib/system/Icon/types';
	import Select from '$lib/system/Select/Select.svelte';
	import Checkbox from '$lib/system/Checkbox/Checkbox.svelte';
	import Icon from '$lib/system/Icon/Icon.svelte';

	type ChartConfigPanelProps = {
		config: GraphConfig;
		columns: string[];
		onConfigChange: (update: Partial<GraphConfig>) => void;
		collapsed: boolean;
		onToggleCollapse: () => void;
	};

	let { config, columns, onConfigChange, collapsed, onToggleCollapse }: ChartConfigPanelProps =
		$props();

	const columnOptions = $derived(columns.map((c) => ({ value: c, label: c })));
	const seriesColumnOptions = $derived([
		{ value: '', label: 'None' },
		...columns.map((c) => ({ value: c, label: c }))
	]);

	const chartTypes: { id: GraphConfig['chartType']; icon: Icons; label: string }[] = [
		{ id: 'bar', icon: 'chart', label: 'Bar' },
		{ id: 'line', icon: 'chart-line', label: 'Line' },
		{ id: 'area', icon: 'chart-line', label: 'Area' }
	];

	// Line never stacks meaningfully with multiple series; bar + area do.
	const supportsStacked = $derived(config.chartType !== 'line');
</script>

<div class="panel" class:collapsed>
	<button class="toggle-btn" onclick={onToggleCollapse} aria-label="Toggle config panel">
		<Icon icon={collapsed ? 'chevron-left' : 'chevron-right'} size={14} />
	</button>

	{#if !collapsed}
		<div class="panel-content scrollable">
			<section>
				<p class="section-label">Chart type</p>
				<div class="chart-type-row">
					{#each chartTypes as ct (ct.id)}
						<button
							class="chart-type-btn"
							class:active={config.chartType === ct.id}
							onclick={() => onConfigChange({ chartType: ct.id })}
							title={ct.label}
						>
							<Icon icon={ct.icon} size={15} />
							<span>{ct.label}</span>
						</button>
					{/each}
				</div>
			</section>

			<section>
				<p class="section-label">X Axis</p>
				<Select
					options={columnOptions}
					value={config.xColumn ?? ''}
					placeholder="Select column"
					size="xs"
					onchange={(v) => onConfigChange({ xColumn: (v as string) || null })}
				/>
			</section>

			<section>
				<p class="section-label">Series</p>
				<Select
					options={seriesColumnOptions}
					value={config.seriesColumn ?? ''}
					placeholder="None"
					size="xs"
					onchange={(v) => onConfigChange({ seriesColumn: (v as string) || null })}
				/>
			</section>

			<section>
				<p class="section-label">Y Axis</p>
				{#if config.seriesColumn}
					<Select
						options={columnOptions}
						value={config.yColumns[0] ?? ''}
						placeholder="Select value column"
						size="xs"
						onchange={(v) => onConfigChange({ yColumns: v ? [v as string] : [] })}
					/>
				{:else}
					<Select
						options={columnOptions}
						value={config.yColumns}
						placeholder="Select columns"
						multiple={true}
						size="xs"
						onchange={(v) => onConfigChange({ yColumns: v as string[] })}
					/>
				{/if}
			</section>

			<section>
				<p class="section-label">Options</p>
				<div class="options-list">
					<Checkbox
						checked={config.showLegend}
						label="Legend"
						size="sm"
						onchange={(v) => onConfigChange({ showLegend: v })}
					/>
					{#if supportsStacked}
						<Checkbox
							checked={config.stacked ?? false}
							label="Stacked"
							size="sm"
							onchange={(v) => onConfigChange({ stacked: v })}
						/>
					{/if}
				</div>
			</section>
		</div>
	{/if}
</div>

<style>
	.panel {
		height: 100%;
		display: flex;
		flex-direction: row;
		border-left: var(--border);
		overflow: hidden;
	}

	.toggle-btn {
		flex-shrink: 0;
		width: 20px;
		display: flex;
		align-items: center;
		justify-content: center;
		background: none;
		border: none;
		border-right: var(--border);
		cursor: pointer;
		color: var(--gray-800);
		padding: 0;
	}

	.toggle-btn:hover {
		background-color: var(--gray-100);
	}

	.panel-content {
		flex: 1;
		overflow-y: auto;
		padding: var(--space-sm);
		display: flex;
		flex-direction: column;
		gap: var(--space-sm);
	}

	section {
		display: flex;
		flex-direction: column;
		gap: var(--space-xs);
	}

	.section-label {
		font-size: var(--fs-xs);
		font-weight: var(--fw-medium);
		color: var(--gray-800);
		text-transform: uppercase;
		letter-spacing: 0.04em;
	}

	.chart-type-row {
		display: flex;
		gap: var(--space-xxs);
	}

	.chart-type-btn {
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 2px;
		padding: var(--space-xs) 0;
		background: var(--gray-100);
		border: var(--border);
		border-radius: var(--br-xs);
		cursor: pointer;
		color: var(--gray-800);
		font-size: 10px;
		transition: background 0.1s;
	}

	.chart-type-btn:hover {
		background: var(--gray-200);
	}

	.chart-type-btn.active {
		background: var(--blue-subtle, var(--gray-200));
		border-color: var(--blue, var(--gray-500));
		color: var(--blue, var(--gray-900));
	}

	.options-list {
		display: flex;
		flex-direction: column;
		gap: var(--space-xs);
	}
</style>
