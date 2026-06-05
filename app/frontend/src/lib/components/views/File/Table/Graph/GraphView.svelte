<script lang="ts">
	import type { graph } from '$lib/wailsjs/go/models';
	import { Query } from '$lib/wailsjs/go/db_client/DbClient';
	import { updateTab, type Tab, type GraphConfig } from '$lib/components/Layout/layoutStore';
	import {
		inferDefaultConfig,
		transformToChartData,
		pivotChartData,
		buildSeriesSpecs,
		MAX_BAR_CATEGORIES,
		MAX_CHART_ROWS,
		MAX_SERIES
	} from './chartUtils';

	import Chart from './Chart.svelte';
	import ChartConfigPanel from './ChartConfigPanel.svelte';
	import TableEmptyState from '../TableEmptyState.svelte';

	type GraphViewProps = {
		tab: Tab;
		effectiveDbId: string | null;
		queryResult: graph.QueryResult | null;
		content: string;
		onRun: () => void;
	};

	let { tab, effectiveDbId, queryResult, content, onRun }: GraphViewProps = $props();

	const PANEL_WIDTH = 260;
	const PANEL_COLLAPSED_WIDTH = 20;

	let containerWidth = $state(0);
	let containerHeight = $state(0);
	let graphLoading = $state(false);

	const tableState = $derived(effectiveDbId ? (tab.file?.tables?.[effectiveDbId] ?? null) : null);

	const panelCollapsed = $derived(tableState?.graphPanelCollapsed ?? false);

	const chartWidth = $derived(
		Math.max(0, containerWidth - (panelCollapsed ? PANEL_COLLAPSED_WIDTH : PANEL_WIDTH))
	);

	function togglePanel() {
		if (!tab.file || !effectiveDbId) return;
		updateTab({
			...tab,
			file: {
				...tab.file,
				tables: {
					...(tab.file.tables ?? {}),
					[effectiveDbId]: {
						...(tab.file.tables?.[effectiveDbId] ?? {}),
						graphPanelCollapsed: !panelCollapsed
					}
				}
			}
		});
	}

	// The full ForExport dataset is refetched on every mount (same pattern as
	// ResultsTable / ExplainTable), so it lives in local component state. No
	// store, no localStorage bloat, no stale cache across tab switches.
	let graphResult = $state<graph.QueryResult | null>(null);
	let fetchedForId = $state<string | null>(null);

	// Config: stored config takes precedence; fall back to auto-inferred defaults
	const defaultConfig: GraphConfig = {
		chartType: 'bar',
		xColumn: null,
		yColumns: [],
		showLegend: false
	};

	const config = $derived.by((): GraphConfig => {
		if (tableState?.graphConfig) return tableState.graphConfig;
		const result = graphResult ?? queryResult;
		if (!result?.columns?.length) return defaultConfig;
		return inferDefaultConfig(result);
	});

	function updateConfig(update: Partial<GraphConfig>) {
		if (!tab.file || !effectiveDbId) return;
		updateTab({
			...tab,
			file: {
				...tab.file,
				tables: {
					...(tab.file.tables ?? {}),
					[effectiveDbId]: {
						...(tab.file.tables?.[effectiveDbId] ?? {}),
						graphConfig: { ...config, ...update }
					}
				}
			}
		});
	}

	// Fetch full dataset via ForExport: true on mount and whenever the upstream
	// queryResult id changes. `fetchedForId` dedupes re-runs of the effect for
	// the same queryResult (eg. panel toggles, config edits).
	$effect(() => {
		const currentResultId = queryResult?.id;
		if (!currentResultId || !queryResult?.columns?.length) return;
		if (fetchedForId === currentResultId) return;
		if (!effectiveDbId || !tab.file?.node || !content) return;

		const snapshotFile = tab.file;
		const snapshotResultId = currentResultId;

		graphLoading = true;
		Query({
			FileID: snapshotFile.node.id,
			Statement: content,
			DbInstanceID: effectiveDbId,
			FolderID: snapshotFile.node.folder_id ?? '',
			ForExport: true,
			RuntimeVars: snapshotFile.runtimeVars ?? {}
		}).then((result) => {
			graphLoading = false;
			graphResult = result;
			fetchedForId = snapshotResultId;
		});
	});

	// In pivot mode, reshape (x, seriesCol, value) rows into one point per x value.
	const pivoted = $derived.by(() => {
		const result = graphResult;
		if (!result?.rows?.length || !config.xColumn || !config.seriesColumn) return null;
		if (config.yColumns.length === 0) return null;
		return pivotChartData(result, config);
	});

	const rawChartData = $derived.by(() => {
		const result = graphResult;
		if (!result?.rows?.length || !config.xColumn) return [];
		if (pivoted) return pivoted.data;
		if (config.yColumns.length === 0) return [];
		return transformToChartData(result, config);
	});

	// Bar charts cap the category count so a 10k-row dataset doesn't render 10k bars.
	const chartData = $derived(
		config.chartType === 'bar' ? rawChartData.slice(0, MAX_BAR_CATEGORIES) : rawChartData
	);

	const rowsTruncated = $derived(
		(graphResult?.rows?.length ?? 0) > MAX_CHART_ROWS ||
			(config.chartType === 'bar' && rawChartData.length > MAX_BAR_CATEGORIES)
	);
	// In pivot mode all series are rendered; truncation only applies to manual multi-Y.
	const seriesTruncated = $derived(!pivoted && config.yColumns.length > MAX_SERIES);

	const seriesSpecs = $derived(buildSeriesSpecs(config, pivoted?.seriesKeys));
	const columns = $derived(graphResult?.columns ?? queryResult?.columns ?? []);

	let hiddenSeries = $state<Set<string>>(new Set());
	const visibleSeries = $derived(seriesSpecs.filter((s) => !hiddenSeries.has(s.key)));

	function toggleSeries(key: string) {
		const next = new Set(hiddenSeries);
		if (next.has(key)) next.delete(key);
		else next.add(key);
		hiddenSeries = next;
	}
</script>

<div class="graph-view" bind:clientWidth={containerWidth} bind:clientHeight={containerHeight}>
	<div class="chart-area">
		{#if graphLoading}
			<div class="chart-placeholder">
				<p class="muted">Loading full dataset…</p>
			</div>
		{:else if !queryResult}
			<TableEmptyState message="No result yet." button={{ onclick: onRun }} />
		{:else if !config.xColumn || (config.yColumns.length === 0 && !config.seriesColumn)}
			<div class="chart-placeholder">
				<p class="muted">Select X and Y columns in the config panel.</p>
			</div>
		{:else if chartData.length === 0}
			<div class="chart-placeholder">
				<p class="muted">No data to display.</p>
			</div>
		{:else}
			<Chart
				data={chartData}
				series={visibleSeries}
				xColumn={config.xColumn}
				width={chartWidth}
				height={containerHeight}
				kind={config.chartType}
				stacked={config.stacked ?? false}
			/>
		{/if}

		{#if rowsTruncated || seriesTruncated}
			<div class="truncation-notice">
				{#if rowsTruncated}Showing first {chartData.length.toLocaleString()} rows.{/if}
				{#if seriesTruncated}First {MAX_SERIES} series shown.{/if}
			</div>
		{/if}

		{#if seriesSpecs.length > 0 && config.showLegend && chartData.length > 0}
			<div class="legend">
				{#each seriesSpecs as s (s.key)}
					<button
						class="legend-item"
						class:hidden={hiddenSeries.has(s.key)}
						onclick={() => toggleSeries(s.key)}
					>
						<span
							class="legend-dot"
							style="background:{hiddenSeries.has(s.key)
								? 'transparent'
								: s.color}; border-color:{s.color}"
						></span>
						<span class="legend-label">{s.label}</span>
					</button>
				{/each}
			</div>
		{/if}
	</div>

	<div class="config-panel-wrapper" class:collapsed={panelCollapsed}>
		<ChartConfigPanel
			{config}
			{columns}
			onConfigChange={updateConfig}
			collapsed={panelCollapsed}
			onToggleCollapse={togglePanel}
		/>
	</div>
</div>

<style>
	.graph-view {
		width: 100%;
		height: 100%;
		display: flex;
		flex-direction: row;
		overflow: hidden;
		background-color: var(--gray-0);
	}

	.chart-area {
		position: relative;
		flex: 1 1 0;
		min-width: 0;
		overflow: hidden;
	}

	.config-panel-wrapper {
		flex-shrink: 0;
		overflow: hidden;
		width: 260px;
		min-width: 260px;
		max-width: 360px;
	}

	.config-panel-wrapper.collapsed {
		width: 20px;
		min-width: 20px;
	}

	.chart-placeholder {
		width: 100%;
		height: 100%;
		display: flex;
		align-items: center;
		justify-content: center;
	}

	.muted {
		color: var(--gray-800);
		font-weight: var(--fw-light);
		font-size: var(--fs-sm);
	}

	.truncation-notice {
		position: absolute;
		top: var(--space-xs);
		left: 50%;
		transform: translateX(-50%);
		font-size: var(--fs-xs);
		color: var(--gray-700);
		background: var(--gray-100);
		border: var(--border);
		border-radius: var(--br-xs);
		padding: var(--space-xxs) var(--space-xs);
		white-space: nowrap;
	}

	.legend {
		position: absolute;
		bottom: var(--space-xxs);
		left: 50%;
		transform: translateX(-50%);
		display: flex;
		gap: var(--space-sm);
		flex-wrap: wrap;
		justify-content: center;
	}

	.legend-item {
		display: flex;
		align-items: center;
		gap: var(--space-xs);
		background: none;
		border: none;
		padding: var(--space-xs);
		border-radius: var(--br-xs);
		cursor: pointer;
		transition: opacity 0.15s;
	}

	.legend-item:hover {
		background-color: var(--gray-100);
	}

	.legend-item.hidden {
		opacity: 0.4;
	}

	.legend-dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		border: 1.5px solid transparent;
		flex-shrink: 0;
		transition: background 0.15s;
	}

	.legend-label {
		font-size: var(--fs-xs);
		color: var(--gray-900);
	}
</style>
