import type { graph } from '$lib/wailsjs/go/models';
import type { GraphConfig } from '$lib/components/Layout/layoutStore';

export type ColumnKind = 'numeric' | 'temporal' | 'categorical';
export type ChartDataPoint = Record<string, unknown>;

export type SeriesSpec = {
	key: string;
	label: string;
	color: string;
};

export const CHART_PALETTE: string[] = [
	'#4f8ef7',
	'#e86f4b',
	'#52c48a',
	'#f0c048',
	'#a78bfa',
	'#f472b6',
	'#34d399',
	'#fb923c'
];

const TEMPORAL_NAME_RE = /date|time|created|updated|ts|dt|day|month|year|period|week/i;
const NUMERIC_NAME_RE =
	/count|sum|avg|total|amount|price|revenue|cost|value|qty|quantity|rate|ratio/i;

export function classifyColumn(name: string, samples: unknown[]): ColumnKind {
	if (TEMPORAL_NAME_RE.test(name)) return 'temporal';

	// Sample up to 20 values
	const subset = samples.slice(0, 20).filter((v) => v !== null && v !== undefined && v !== '');
	if (subset.length === 0) return 'categorical';

	let numericCount = 0;
	let temporalCount = 0;

	for (const v of subset) {
		const str = String(v);
		if (!isNaN(Number(str)) && str.trim() !== '') {
			numericCount++;
		} else {
			const d = new Date(str);
			if (!isNaN(d.getTime()) && str.length > 4) temporalCount++;
		}
	}

	const ratio = subset.length;
	if (numericCount / ratio >= 0.7) return 'numeric';
	if (temporalCount / ratio >= 0.5) return 'temporal';
	return 'categorical';
}

// Detect the (time, category, value) pivot pattern.
// Returns the categorical column to use as the series dimension, or null.
function inferSeriesColumn(
	result: graph.QueryResult,
	columns: string[],
	kinds: ColumnKind[],
	xColumn: string
): string | null {
	const catCols = columns.filter((c, i) => kinds[i] === 'categorical' && c !== xColumn);
	const numCols = columns.filter((c, i) => kinds[i] === 'numeric' && c !== xColumn);

	if (catCols.length !== 1 || numCols.length === 0) return null;

	const rows = (result.rows ?? []) as unknown[][];
	if (rows.length < 2) return null;

	const sample = rows.slice(0, 200);
	const xIdx = columns.indexOf(xColumn);
	const catIdx = columns.indexOf(catCols[0]);

	const distinctX = new Set(sample.map((r) => String(r[xIdx] ?? '')));
	const distinctCat = new Set(sample.map((r) => String(r[catIdx] ?? '')));

	// Multiple categories across multiple x values = pivot pattern
	if (distinctCat.size > 1 && distinctX.size > 1) return catCols[0];
	return null;
}

export function inferDefaultConfig(result: graph.QueryResult): GraphConfig {
	const columns = result.columns ?? [];
	const rows = result.rows ?? [];

	if (columns.length === 0) {
		return { chartType: 'bar', xColumn: null, yColumns: [], showLegend: false };
	}

	const kinds = columns.map((col, i) => {
		const samples = rows.slice(0, 20).map((row) => (row as unknown[])[i]);
		return classifyColumn(col, samples);
	});

	// Pick xColumn: prefer temporal, then first categorical
	let xColumn: string | null = null;
	let chartType: 'bar' | 'line' | 'area' = 'bar';

	const temporalIdx = kinds.findIndex((k) => k === 'temporal');
	if (temporalIdx !== -1) {
		xColumn = columns[temporalIdx];
		chartType = 'line';
	} else {
		const catIdx = kinds.findIndex((k) => k === 'categorical');
		xColumn = catIdx !== -1 ? columns[catIdx] : columns[0];
	}

	// Detect pivot pattern: one categorical + one numeric beside a temporal x
	const seriesColumn =
		chartType === 'line' ? inferSeriesColumn(result, columns, kinds, xColumn!) : null;

	if (seriesColumn) {
		const valueCol = columns.find(
			(c, i) => kinds[i] === 'numeric' && c !== xColumn && c !== seriesColumn
		);
		return {
			chartType: 'line',
			xColumn,
			yColumns: valueCol ? [valueCol] : [],
			seriesColumn,
			showLegend: true
		};
	}

	// Normal mode: all numeric cols as Y series
	const yColumns = columns.filter((col, i) => kinds[i] === 'numeric' && col !== xColumn);

	if (yColumns.length === 0) {
		const byName = columns.filter((col) => NUMERIC_NAME_RE.test(col) && col !== xColumn);
		if (byName.length > 0) yColumns.push(...byName);
	}
	if (yColumns.length === 0) {
		yColumns.push(...columns.filter((col) => col !== xColumn));
	}

	return { chartType, xColumn, yColumns, showLegend: false };
}

export const MAX_CHART_ROWS = 10_000;
export const MAX_BAR_CATEGORIES = 300;
export const MAX_SERIES = 20;

export function transformToChartData(
	result: graph.QueryResult,
	config: GraphConfig
): ChartDataPoint[] {
	const columns = result.columns ?? [];
	const rows = (result.rows ?? []).slice(0, MAX_CHART_ROWS) as unknown[][];

	return rows.map((row) => {
		const point: ChartDataPoint = {};
		columns.forEach((col, i) => {
			const val = row[i];
			if (config.yColumns.includes(col)) {
				const n = Number(val);
				point[col] = isNaN(n) ? 0 : n;
			} else {
				point[col] = val;
			}
		});
		return point;
	});
}

// Reshape (x, seriesCol, valueCol) rows into one ChartDataPoint per x value,
// with each distinct series value as a key.
export function pivotChartData(
	result: graph.QueryResult,
	config: GraphConfig
): { data: ChartDataPoint[]; seriesKeys: string[] } {
	const columns = result.columns ?? [];
	const rows = (result.rows ?? []).slice(0, MAX_CHART_ROWS) as unknown[][];

	const xIdx = columns.indexOf(config.xColumn!);
	const seriesIdx = columns.indexOf(config.seriesColumn!);
	const valueCol = config.yColumns[0];
	const yIdx = valueCol !== undefined ? columns.indexOf(valueCol) : -1;

	if (xIdx === -1 || seriesIdx === -1 || yIdx === -1) return { data: [], seriesKeys: [] };

	const seriesKeySet = new Set<string>();
	const xOrder: string[] = [];
	const xSeen = new Set<string>();
	const xRaw = new Map<string, unknown>();
	// xKey -> seriesKey -> value
	const lookup = new Map<string, Map<string, number>>();

	for (const row of rows) {
		const xVal = row[xIdx];
		const xKey = String(xVal ?? '');
		const sk = String(row[seriesIdx] ?? '');
		const n = Number(row[yIdx]);

		seriesKeySet.add(sk);
		if (!xSeen.has(xKey)) {
			xSeen.add(xKey);
			xOrder.push(xKey);
			xRaw.set(xKey, xVal);
			lookup.set(xKey, new Map());
		}
		lookup.get(xKey)!.set(sk, isNaN(n) ? 0 : n);
	}

	const seriesKeys = [...seriesKeySet];

	const data: ChartDataPoint[] = xOrder.map((xKey) => {
		const point: ChartDataPoint = { [config.xColumn!]: xRaw.get(xKey) };
		const seriesMap = lookup.get(xKey)!;
		for (const sk of seriesKeys) {
			point[sk] = seriesMap.get(sk) ?? null;
		}
		return point;
	});

	return { data, seriesKeys };
}

export function buildSeriesSpecs(config: GraphConfig, pivotKeys?: string[]): SeriesSpec[] {
	// In pivot mode, never cap, the category count is data-driven.
	// In manual mode, cap at MAX_SERIES (user explicitly chose those columns).
	const keys = pivotKeys ?? config.yColumns.slice(0, MAX_SERIES);
	return keys.map((key, i) => ({
		key,
		label: key,
		color: CHART_PALETTE[i % CHART_PALETTE.length]
	}));
}
