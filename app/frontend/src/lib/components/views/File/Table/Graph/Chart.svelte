<script lang="ts">
	import * as d3 from 'd3';
	import type { ChartDataPoint, SeriesSpec } from './chartUtils';
	import { tryCatch } from '$lib/utils/tryCatch';

	export type ChartKind = 'bar' | 'line' | 'area';

	type Props = {
		data: ChartDataPoint[];
		series: SeriesSpec[];
		xColumn: string;
		width: number;
		height: number;
		kind: ChartKind;
		stacked?: boolean;
	};

	let { data, series, xColumn, width, height, kind, stacked = false }: Props = $props();

	const MARGIN = { top: 20, right: 20, bottom: 24, left: 40 };
	const DOT_THRESHOLD = 200;
	const TRANSITION_MS = 300;
	const TOOLTIP_MAX = 10;

	// CSS class names can't contain spaces or special chars, sanitize series keys.
	const cssKey = (k: string) => k.replace(/[^a-zA-Z0-9_-]/g, '_');

	let svgEl = $state<SVGSVGElement | null>(null);
	let tooltipEl = $state<HTMLDivElement | null>(null);

	function hideTip() {
		if (tooltipEl) d3.select(tooltipEl).style('opacity', '0');
	}

	$effect(() => {
		if (!svgEl || width <= 0 || height <= 0 || data.length === 0 || series.length === 0) return;

		const svg = d3.select(svgEl);
		svg.selectAll('*').remove();

		const [, err] = tryCatch(() => {
			const innerW = width - MARGIN.left - MARGIN.right;
			const innerH = height - MARGIN.top - MARGIN.bottom;
			if (innerW <= 0 || innerH <= 0) return;

			const root = svg
				.append('g')
				.attr('transform', `translate(${MARGIN.left},${MARGIN.top})`) as d3.Selection<
				SVGGElement,
				unknown,
				null,
				undefined
			>;

			const keys = series.map((s) => s.key);
			const xValues = data.map((d: ChartDataPoint) => String(d[xColumn] ?? ''));
			const colorByKey = Object.fromEntries(series.map((s) => [s.key, s.color]));

			// ---- x scale + x axis ----
			// Bar => band; line/area => time if the first x value parses as a date, else point.
			let getX: (d: ChartDataPoint) => number;
			let xAxis: d3.Axis<d3.AxisDomain>;
			let band: d3.ScaleBand<string> | null = null;

			if (kind === 'bar') {
				band = d3.scaleBand<string>().domain(xValues).range([0, innerW]).padding(0.2);
				getX = (d: ChartDataPoint) => band!(String(d[xColumn] ?? '')) ?? 0;
				xAxis = d3
					.axisBottom(band)
					.tickSizeOuter(0)
					.tickFormat(() => '') as d3.Axis<d3.AxisDomain>;
			} else {
				const firstVal = xValues.find((v) => v !== '') ?? '';
				const isTime =
					!isNaN(new Date(firstVal).getTime()) && isNaN(Number(firstVal)) && firstVal.length > 4;
				if (isTime) {
					const ts = d3
						.scaleTime()
						.domain(
							d3.extent(data.map((d: ChartDataPoint) => new Date(String(d[xColumn] ?? '')))) as [
								Date,
								Date
							]
						)
						.range([0, innerW]);
					getX = (d: ChartDataPoint) => ts(new Date(String(d[xColumn] ?? ''))) ?? 0;
					xAxis = d3
						.axisBottom(ts)
						.ticks(6)
						.tickSizeOuter(0)
						.tickFormat(() => '') as d3.Axis<d3.AxisDomain>;
				} else {
					const ps = d3.scalePoint<string>().domain(xValues).range([0, innerW]).padding(0.5);
					getX = (d: ChartDataPoint) => ps(String(d[xColumn] ?? '')) ?? 0;
					xAxis = d3
						.axisBottom(ps)
						.tickSizeOuter(0)
						.tickFormat(() => '') as d3.Axis<d3.AxisDomain>;
				}
			}

			// ---- y scale ----
			// Line never stacks; bar + area honor `stacked`.
			const isStacked = stacked && kind !== 'line';
			let yMax: number;
			if (isStacked) {
				const sd = d3.stack<ChartDataPoint>().keys(keys)(data);
				yMax = d3.max(sd.at(-1) ?? [], (d: d3.SeriesPoint<ChartDataPoint>) => d[1]) ?? 0;
			} else {
				yMax =
					d3.max(data, (row: ChartDataPoint) =>
						d3.max(keys, (k: string) => (typeof row[k] === 'number' ? (row[k] as number) : 0))
					) ?? 0;
			}
			const yScale = d3
				.scaleLinear()
				.domain([0, yMax * 1.05])
				.range([innerH, 0])
				.nice();

			// ---- grid + axes (shared across all kinds) ----
			root
				.append('g')
				.attr('class', 'grid')
				.call(
					d3
						.axisLeft(yScale)
						.tickSize(-innerW)
						.tickFormat(() => '')
				)
				.call((g: d3.Selection<SVGGElement, unknown, null, undefined>) =>
					g.select('.domain').remove()
				)
				.call((g: d3.Selection<SVGGElement, unknown, null, undefined>) =>
					g
						.selectAll('.tick line')
						.attr('stroke', 'var(--border-color)')
						.attr('stroke-dasharray', '2,3')
				);

			root
				.append('g')
				.attr('transform', `translate(0,${innerH})`)
				.call(xAxis)
				.call((g: d3.Selection<SVGGElement, unknown, null, undefined>) => {
					g.select('.domain').attr('stroke', 'var(--border-color)');
					g.selectAll('.tick line').attr('stroke', 'var(--border-color)');
				});

			root
				.append('g')
				.call(d3.axisLeft(yScale).ticks(5).tickSizeOuter(0))
				.call((g: d3.Selection<SVGGElement, unknown, null, undefined>) => {
					g.select('.domain').attr('stroke', 'var(--border-color)');
					g.selectAll('.tick line').attr('stroke', 'var(--border-color)');
					g.selectAll('text').attr('fill', 'var(--gray-900)').attr('font-size', '11px');
				});

			// Snap cursor to nearest data point by pixel distance.
			const snapTo = (mx: number): number => {
				let best = 0,
					minD = Infinity;
				data.forEach((d, i) => {
					const dist = Math.abs(getX(d) - mx);
					if (dist < minD) {
						minD = dist;
						best = i;
					}
				});
				return best;
			};

			// Tooltip content: sorted by value desc, truncated, closest-to-cursor-y highlighted.
			// focusKey overrides the auto y-distance focus (used for stacked bars where raw values
			// don't correspond to screen positions).
			const buildTooltip = (row: ChartDataPoint, my: number, focusKey?: string | null): string => {
				const xVal = String(row[xColumn] ?? '');
				const entries = series
					.map((s) => ({ s, v: typeof row[s.key] === 'number' ? (row[s.key] as number) : null }))
					.sort((a, b) => {
						if (a.v === null) return 1;
						if (b.v === null) return -1;
						return b.v - a.v;
					});
				const focus =
					focusKey != null
						? (entries.find((e) => e.s.key === focusKey) ?? null)
						: (() => {
								const yCursor = yScale.invert(my);
								return entries.reduce<(typeof entries)[0] | null>((best, cur) => {
									if (cur.v === null) return best;
									if (!best || Math.abs(cur.v - yCursor) < Math.abs(best.v! - yCursor)) return cur;
									return best;
								}, null);
							})();
				const shown = entries.slice(0, TOOLTIP_MAX);
				const extra = entries.length - TOOLTIP_MAX;
				const rows = shown
					.map(
						({ s, v }) =>
							`<div class="tr${s.key === focus?.s.key ? ' tf' : ''}">` +
							`<span class="td" style="background:${s.color}"></span>` +
							`<span class="tl">${s.label}</span>` +
							`<span class="tv">${v !== null ? v : '–'}</span>` +
							`</div>`
					)
					.join('');
				const more = extra > 0 ? `<div class="te">+${extra} more</div>` : '';
				return `<div class="tx">${xVal}</div><div class="tb">${rows}${more}</div>`;
			};

			// Position tooltip at the top corner opposite the cursor:
			// mouse on left → top-right, mouse on right → top-left.
			const positionTooltip = (mx: number): void => {
				if (!tooltipEl || !svgEl) return;
				const rect = svgEl.getBoundingClientRect();
				const tipW = tooltipEl.offsetWidth;
				const tipH = tooltipEl.offsetHeight;
				const MARGIN_SCREEN = 8;

				let left =
					mx > innerW / 2 ? rect.left + MARGIN.left + 12 : rect.left + MARGIN.left + innerW - tipW;
				let top = rect.top + MARGIN.top + 8;

				// Clamp to viewport
				left = Math.min(left, window.innerWidth - tipW - MARGIN_SCREEN);
				left = Math.max(left, MARGIN_SCREEN);
				top = Math.min(top, window.innerHeight - tipH - MARGIN_SCREEN);
				top = Math.max(top, MARGIN_SCREEN);

				d3.select(tooltipEl).style('left', `${left}px`).style('top', `${top}px`);
			};

			// Crosshair, drawn on top of marks, below markers and overlay.
			const crosshair = root
				.append('line')
				.attr('y1', 0)
				.attr('y2', innerH)
				.attr('stroke', 'var(--gray-700)')
				.attr('stroke-width', 1)
				.attr('pointer-events', 'none')
				.style('opacity', '0');

			// ---- marks ----
			if (kind === 'bar') {
				const t = d3.transition().duration(TRANSITION_MS).ease(d3.easeCubicOut);

				// Hoisted so the overlay mousemove can use it for focus detection.
				let stackKeys: string[] | null = null;

				if (isStacked) {
					// Sort by total contribution ascending: smallest sum at the bottom, largest at the top.
					stackKeys = [...keys].sort(
						(a, b) =>
							(d3.max(data, (d: ChartDataPoint) =>
								typeof d[a] === 'number' ? (d[a] as number) : 0
							) ?? 0) -
							(d3.max(data, (d: ChartDataPoint) =>
								typeof d[b] === 'number' ? (d[b] as number) : 0
							) ?? 0)
					);
					const sd = d3.stack<ChartDataPoint>().keys(stackKeys)(data);
					sd.forEach((layer: d3.Series<ChartDataPoint, string>) => {
						const ck = cssKey(layer.key);
						/* eslint-disable @typescript-eslint/no-explicit-any */
						root
							.selectAll(`.bar-${ck}`)
							.data(layer)
							.join(
								(enter: any) =>
									enter
										.append('rect')
										.attr('class', `bar-${ck}`)
										.attr('x', (_: any, i: number) => band!(xValues[i]) ?? 0)
										.attr('y', innerH)
										.attr('width', band!.bandwidth())
										.attr('height', 0)
										.attr('fill', colorByKey[layer.key] ?? '#888')
										.attr('rx', 2),
								(u: any) => u,
								(e: any) => e.remove()
							)
							/* eslint-enable @typescript-eslint/no-explicit-any */
							.transition(t)
							.attr('x', (_: d3.SeriesPoint<ChartDataPoint>, i: number) => band!(xValues[i]) ?? 0)
							.attr('y', (d: d3.SeriesPoint<ChartDataPoint>) => yScale(d[1]))
							.attr('width', band!.bandwidth())
							.attr('height', (d: d3.SeriesPoint<ChartDataPoint>) =>
								Math.max(0, yScale(d[0]) - yScale(d[1]))
							);
					});
				} else {
					const innerX = d3.scaleBand().domain(keys).range([0, band!.bandwidth()]).padding(0.05);
					series.forEach((s) => {
						const ck = cssKey(s.key);
						/* eslint-disable @typescript-eslint/no-explicit-any */
						root
							.selectAll(`.bar-${ck}`)
							.data(data)
							.join(
								(enter: any) =>
									enter
										.append('rect')
										.attr('class', `bar-${ck}`)
										.attr(
											'x',
											(d: any) => (band!(String(d[xColumn] ?? '')) ?? 0) + (innerX(s.key) ?? 0)
										)
										.attr('y', innerH)
										.attr('width', innerX.bandwidth())
										.attr('height', 0)
										.attr('fill', s.color)
										.attr('rx', 2),
								(u: any) => u,
								(e: any) => e.remove()
							)
							/* eslint-enable @typescript-eslint/no-explicit-any */
							.transition(t)
							.attr(
								'x',
								(d: ChartDataPoint) => (band!(String(d[xColumn] ?? '')) ?? 0) + (innerX(s.key) ?? 0)
							)
							.attr('y', (d: ChartDataPoint) =>
								yScale(typeof d[s.key] === 'number' ? (d[s.key] as number) : 0)
							)
							.attr('width', innerX.bandwidth())
							.attr(
								'height',
								(d: ChartDataPoint) =>
									innerH - yScale(typeof d[s.key] === 'number' ? (d[s.key] as number) : 0)
							);
					});
				}

				root
					.append('rect')
					.attr('width', innerW)
					.attr('height', innerH)
					.attr('fill', 'none')
					.attr('pointer-events', 'all')
					.on('mousemove', (e: MouseEvent) => {
						const [mx, my] = d3.pointer(e);
						const idx = snapTo(mx);
						const row = data[idx];
						// Center crosshair on the bar group, not the band left edge.
						const snapX = getX(row) + band!.bandwidth() / 2;
						crosshair.attr('x1', snapX).attr('x2', snapX).style('opacity', '1');
						if (tooltipEl) {
							// For stacked bars, find which segment the cursor y falls within by walking
							// cumulative sums bottom-to-top (stackKeys is sorted ascending = bottom first).
							let focusKey: string | null = null;
							if (stackKeys) {
								const cursorVal = yScale.invert(my);
								let cum = 0;
								for (const k of stackKeys) {
									cum += typeof row[k] === 'number' ? (row[k] as number) : 0;
									if (cursorVal <= cum) {
										focusKey = k;
										break;
									}
								}
								// Dim all segments, highlight the hovered one.
								root.selectAll('[class^="bar-"]').style('opacity', '0.3');
								if (focusKey) root.selectAll(`.bar-${cssKey(focusKey)}`).style('opacity', '1');
							}
							d3.select(tooltipEl)
								.html(buildTooltip(row, my, focusKey))
								.style('opacity', '1');
							positionTooltip(mx);
						}
					})
					.on('mouseleave', () => {
						crosshair.style('opacity', '0');
						if (stackKeys) root.selectAll('[class^="bar-"]').style('opacity', null);
						hideTip();
					});
			} else {
				// line + area
				const clipId = `clip-${Math.random().toString(36).slice(2)}`;
				svg
					.append('defs')
					.append('clipPath')
					.attr('id', clipId)
					.append('rect')
					.attr('width', innerW)
					.attr('height', innerH);
				const chartArea = root.append('g').attr('clip-path', `url(#${clipId})`) as d3.Selection<
					SVGGElement,
					unknown,
					null,
					undefined
				>;

				const showArea = kind === 'area';

				// Hoisted so the overlay mousemove can walk cumulative sums for focus detection.
				let areaStackKeys: string[] | null = null;

				if (isStacked && showArea) {
					// Same ordering as stacked bars: lowest total contribution at bottom, highest at top.
					areaStackKeys = [...keys].sort(
						(a, b) =>
							(d3.max(data, (d: ChartDataPoint) =>
								typeof d[a] === 'number' ? (d[a] as number) : 0
							) ?? 0) -
							(d3.max(data, (d: ChartDataPoint) =>
								typeof d[b] === 'number' ? (d[b] as number) : 0
							) ?? 0)
					);
					const sd = d3.stack<ChartDataPoint>().keys(areaStackKeys)(data);
					const areaGen = d3
						.area<d3.SeriesPoint<ChartDataPoint>>()
						.x((_: d3.SeriesPoint<ChartDataPoint>, i: number) => getX(data[i]))
						.y0((d: d3.SeriesPoint<ChartDataPoint>) => yScale(d[0]))
						.y1((d: d3.SeriesPoint<ChartDataPoint>) => yScale(d[1]))
						.curve(d3.curveMonotoneX);
					sd.forEach((layer: d3.Series<ChartDataPoint, string>) => {
						chartArea
							.append('path')
							.datum(layer)
							.attr('class', 'area-layer')
							.attr('data-key', layer.key)
							.attr('fill', colorByKey[layer.key] ?? '#888')
							.attr('fill-opacity', 0.6)
							.attr('d', areaGen);
					});
				} else {
					series.forEach((s) => {
						if (showArea) {
							const areaGen = d3
								.area<ChartDataPoint>()
								.x((d: ChartDataPoint) => getX(d))
								.y0(innerH)
								.y1((d: ChartDataPoint) =>
									yScale(typeof d[s.key] === 'number' ? (d[s.key] as number) : 0)
								)
								.curve(d3.curveMonotoneX);
							chartArea
								.append('path')
								.datum(data)
								.attr('fill', s.color)
								.attr('fill-opacity', 0.15)
								.attr('d', areaGen);
						}
						const lineGen = d3
							.line<ChartDataPoint>()
							.x((d: ChartDataPoint) => getX(d))
							.y((d: ChartDataPoint) =>
								yScale(typeof d[s.key] === 'number' ? (d[s.key] as number) : 0)
							)
							.curve(d3.curveMonotoneX);
						chartArea
							.append('path')
							.datum(data)
							.attr('fill', 'none')
							.attr('stroke', s.color)
							.attr('stroke-width', 2)
							.attr('d', lineGen);

						if (data.length <= DOT_THRESHOLD) {
							const dck = cssKey(s.key);
							chartArea
								.selectAll(`.dot-${dck}`)
								.data(data)
								.join('circle')
								.attr('class', `dot-${dck}`)
								.attr('cx', (d: ChartDataPoint) => getX(d))
								.attr('cy', (d: ChartDataPoint) =>
									yScale(typeof d[s.key] === 'number' ? (d[s.key] as number) : 0)
								)
								.attr('r', 3)
								.attr('fill', s.color)
								.attr('stroke', 'var(--gray-0)')
								.attr('stroke-width', 1.5);
						}
					});
				}

				// Marker dots, only for line and non-stacked area.
				// Stacked area draws series at cumulative y, so raw values don't map to screen positions.
				const showMarkers = !isStacked;
				const dotGroup = root.append('g').attr('pointer-events', 'none');
				const markerDots = showMarkers
					? series.map((s) =>
							dotGroup
								.append('circle')
								.attr('r', 4.5)
								.attr('fill', s.color)
								.attr('stroke', 'var(--gray-0)')
								.attr('stroke-width', 2)
								.style('opacity', '0')
						)
					: [];

				root
					.append('rect')
					.attr('width', innerW)
					.attr('height', innerH)
					.attr('fill', 'none')
					.attr('pointer-events', 'all')
					.on('mousemove', (e: MouseEvent) => {
						const [mx, my] = d3.pointer(e);
						const idx = snapTo(mx);
						const row = data[idx];
						const snapX = getX(row);
						crosshair.attr('x1', snapX).attr('x2', snapX).style('opacity', '1');
						markerDots.forEach((dot, i) => {
							const v = row[series[i].key];
							const yVal = typeof v === 'number' ? v : null;
							if (yVal !== null) {
								dot.attr('cx', snapX).attr('cy', yScale(yVal)).style('opacity', '1');
							} else {
								dot.style('opacity', '0');
							}
						});
						if (tooltipEl) {
							let focusKey: string | null = null;
							if (areaStackKeys) {
								const cursorVal = yScale.invert(my);
								let cum = 0;
								for (const k of areaStackKeys) {
									cum += typeof row[k] === 'number' ? (row[k] as number) : 0;
									if (cursorVal <= cum) {
										focusKey = k;
										break;
									}
								}
								// Dim all layers, highlight the hovered one.
								root.selectAll('.area-layer').style('opacity', '0.25');
								if (focusKey)
									root.selectAll(`.area-layer[data-key="${focusKey}"]`).style('opacity', '1');
							}
							d3.select(tooltipEl)
								.html(buildTooltip(row, my, focusKey))
								.style('opacity', '1');
							positionTooltip(mx);
						}
					})
					.on('mouseleave', () => {
						crosshair.style('opacity', '0');
						markerDots.forEach((dot) => dot.style('opacity', '0'));
						if (areaStackKeys) root.selectAll('.area-layer').style('opacity', null);
						hideTip();
					});
			}
		});
		if (err) {
			console.error('[Chart]', err.message);
			svg.selectAll('*').remove();
		}
	});
</script>

<div class="chart-wrapper">
	<svg bind:this={svgEl} {width} {height}></svg>
	<div class="tooltip" bind:this={tooltipEl}></div>
</div>

<style>
	.chart-wrapper {
		position: relative;
		width: 100%;
		height: 100%;
	}
	svg {
		display: block;
		overflow: visible;
	}
	.tooltip {
		position: fixed;
		pointer-events: none;
		background-color: var(--gray-0);
		color: var(--gray-900);
		border: var(--border);
		border-radius: var(--br-xs);
		font-size: 12px;
		opacity: 0;
		transition: opacity 0.1s;
		white-space: nowrap;
		z-index: 9999;
		min-width: 160px;
		max-width: 260px;
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
	}
	.tooltip :global(.tx) {
		padding: var(--space-xs) var(--space-sm);
		font-size: 11px;
		color: var(--gray-700);
		border-bottom: var(--border);
		font-weight: var(--fw-medium);
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.tooltip :global(.tb) {
		padding: var(--space-xs) 0;
	}
	.tooltip :global(.tr) {
		display: flex;
		align-items: center;
		gap: var(--space-xs);
		padding: 2px var(--space-sm);
		border-radius: 2px;
	}
	.tooltip :global(.tf) {
		background-color: var(--gray-100);
	}
	.tooltip :global(.tf) :global(.tl) {
		color: var(--gray-1000);
	}
	.tooltip :global(.tf) :global(.tv) {
		color: var(--gray-1000);
	}
	.tooltip :global(.td) {
		width: 7px;
		height: 7px;
		border-radius: 50%;
		flex-shrink: 0;
	}
	.tooltip :global(.tl) {
		flex: 1;
		overflow: hidden;
		text-overflow: ellipsis;
		color: var(--gray-800);
	}
	.tooltip :global(.tv) {
		font-weight: var(--fw-medium);
		color: var(--gray-900);
		margin-left: var(--space-xs);
	}
	.tooltip :global(.te) {
		padding: 2px var(--space-sm);
		font-size: 11px;
		color: var(--gray-700);
	}
</style>
