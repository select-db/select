<script lang="ts">
	import { tick, untrack } from 'svelte';
	import type { Component } from 'svelte';

	import * as graph from '$lib/wails/graph';

	import Button from '$lib/system/Button/Button.svelte';
	import Group from '$lib/system/Group/Group.svelte';

	import { updateTab, type Tab, type TableEdit } from '$lib/components/Layout/layoutStore';
	import EditableCell from './EditableCell.svelte';

	import {
		DEFAULT_ROW_HEIGHT,
		createViewportObserver,
		createRowMeasurer,
		getVisibleRows,
		getTopSpacerHeight,
		getBottomSpacerHeight,
		resetVisibleRange,
		type VirtualScrollState,
		type VisibleRange
	} from './helpers/virtualScrolling';
	import {
		togglePinColumn as togglePinColumnFn,
		getColumnStyles as getColumnStylesFn,
		getColumnWidth as getColumnWidthFn,
		getVisibleColumnRange,
		resetColumnState,
		type ColumnState
	} from './helpers/columnManagement';
	import {
		createInitialLoadingStateFromResult,
		mergeNewPageIntoState,
		createEmptyLoadingState,
		loadMissingPagesForVisibleRange,
		type DataLoadingState
	} from './helpers/dataLoading';
	import { debounce } from '$lib/utils/debounce';
	import { throttle } from '$lib/utils/throttle';
	import { getEffectiveSelectedDbId, getQueryResultForDb } from '../views/tableViewState';
	import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
	import { modalStore } from '$lib/system/Modal/ModalStore';
	import ItemInfoModal from '$lib/components/views/FileSystem/modals/ItemInfoModal.svelte';
	import CellValueModal from './CellValueModal.svelte';
	import ForeignKeyPickerModal from './ForeignKeyPickerModal.svelte';
	import type { ForeignKeyContext } from './EditableCell.svelte';
	import { getExecution } from '$lib/utils/query/queryStream.svelte';

	function findTableNode(
		ws: graph.WorkspaceNode | undefined,
		databaseId: string,
		schema: string,
		table: string
	): graph.DBInstanceItemNode | null {
		if (!ws) return null;
		const db = ws.db_instances.find((d) => d.id === databaseId);
		if (!db) return null;
		const schemaNode = db.children.find((c) => c.type === 'schema' && c.name === schema);
		if (!schemaNode) return null;
		const tablesGroup = schemaNode.children.find((c) => c.type === 'tables');
		const viewsGroup = schemaNode.children.find((c) => c.type === 'views');
		return (
			(tablesGroup?.children ?? []).find((c) => c.name === table) ??
			(viewsGroup?.children ?? []).find((c) => c.name === table) ??
			null
		);
	}

	function findColumnNode(
		ws: graph.WorkspaceNode | undefined,
		meta: graph.ColumnMetadata
	): graph.DBInstanceItemNode | null {
		if (!ws || !meta.databaseId || !meta.schema || !meta.table || !meta.originalColumnName) {
			return null;
		}
		const tableNode = findTableNode(ws, meta.databaseId, meta.schema, meta.table);
		if (!tableNode) return null;
		const columnsGroup = tableNode.children.find((c) => c.type === 'columns');
		return (columnsGroup?.children ?? []).find((c) => c.name === meta.originalColumnName) ?? null;
	}

	type FkRef = { schemaName: string; tableName: string; columnName: string };
	function readForeignKey(node: graph.DBInstanceItemNode | null): FkRef | null {
		const meta = (node?.metadata ?? null) as { foreignKey?: FkRef } | null;
		const fk = meta?.foreignKey;
		if (!fk || !fk.schemaName || !fk.tableName || !fk.columnName) return null;
		return fk;
	}

	function tableColumnNames(node: graph.DBInstanceItemNode | null): string[] {
		const columnsGroup = (node?.children ?? []).find((c) => c.type === 'columns');
		return (columnsGroup?.children ?? []).map((c) => c.name ?? '').filter(Boolean);
	}

	// Pick a reasonable default display column when the user hasn't configured one.
	const DISPLAY_HINTS = ['name', 'title', 'label', 'email', 'username', 'slug'];
	function pickHeuristicColumn(cols: string[], fkCol: string): string | null {
		const lower = cols.map((c) => c.toLowerCase());
		for (const hint of DISPLAY_HINTS) {
			const i = lower.indexOf(hint);
			if (i >= 0) return cols[i];
		}
		// First column that isn't the FK column.
		const first = cols.find((c) => c !== fkCol);
		return first ?? null;
	}

	const columnNodes = $derived.by(() => {
		const ws = $workspaceGraphStore;
		return columnMetadataArray.map((meta) => (meta ? findColumnNode(ws, meta) : null));
	});

	type Props = {
		tab: Tab;
	};

	let { tab }: Props = $props();

	const file = $derived(tab.file?.node);
	const effectiveDbId = $derived(getEffectiveSelectedDbId(file, tab));
	const queryResult = $derived(getQueryResultForDb(file, effectiveDbId));

	const tableState = $derived.by(() => {
		if (!tab.file) return null;
		if (!effectiveDbId) return null;
		return tab.file.tables?.[effectiveDbId] ?? null;
	});

	const edits = $derived(tableState?.edits ?? {});

	// Hierarchical edit storage - check row first, then column
	// Most rows have no edits, so we skip column checks entirely
	const editsByRow = $derived.by(() => {
		// eslint-disable-next-line svelte/prefer-svelte-reactivity -- local map built and returned inside a derivation
		const rows = new Map<number, Map<number, string>>();
		for (const [key, edit] of Object.entries(edits)) {
			const [rowStr, colStr] = key.split(':');
			const rowIndex = parseInt(rowStr, 10);
			const colIndex = parseInt(colStr, 10);
			if (!rows.has(rowIndex)) {
				rows.set(rowIndex, new Map());
			}
			rows.get(rowIndex)!.set(colIndex, edit.value);
		}
		return rows;
	});

	const columnMetadataArray = $derived.by(() => {
		return queryResult?.columnMetadata ?? [];
	});

	// For each column, the list of primary key column names missing from the SELECT.
	const missingPrimaryKeysPerColumn = $derived.by(() => {
		return columnMetadataArray.map((cm) => {
			if (!cm || cm.hasAllPrimaryKeys) return [];

			const required = cm.primaryKeys ?? [];
			if (!required.length) return [];

			const present = new Set(
				columnMetadataArray
					.filter(
						(other) =>
							other?.isPrimaryKey && other?.table === cm.table && other?.schema === cm.schema
					)
					.map((other) => other.originalColumnName?.toLowerCase())
					.filter(Boolean) as string[]
			);

			return required.filter((pk) => !present.has(pk.toLowerCase()));
		});
	});

	// Per column: the FK picker context. Null when the column isn't a FK or when
	// we couldn't resolve the target table from the workspace graph.
	const foreignKeyContexts = $derived.by(() => {
		const ws = $workspaceGraphStore;
		const dbId = effectiveDbId;
		const persisted = tableState?.foreignKeyDisplayColumns ?? {};

		return columnMetadataArray.map((meta, i) => {
			if (!meta?.isForeignKey || !dbId) return null;
			const fk = readForeignKey(columnNodes[i]);
			if (!fk) return null;
			const targetTable = findTableNode(ws, dbId, fk.schemaName, fk.tableName);
			const available = tableColumnNames(targetTable);
			if (available.length === 0) return null;

			const sourceCol = meta.originalColumnName ?? '';
			const userSelected = persisted[sourceCol];
			let selected: string[];
			if (userSelected && userSelected.length > 0) {
				selected = userSelected.filter((c) => available.includes(c));
				if (selected.length === 0) selected = [fk.columnName];
			} else {
				const heuristic = pickHeuristicColumn(available, fk.columnName);
				selected =
					heuristic && heuristic !== fk.columnName ? [fk.columnName, heuristic] : [fk.columnName];
			}

			return {
				databaseId: dbId,
				targetSchema: fk.schemaName,
				targetTable: fk.tableName,
				targetColumn: fk.columnName,
				availableColumns: available,
				selectedColumns: selected,
				sourceColumn: sourceCol
			};
		});
	});

	function updateForeignKeyDisplayColumns(sourceColumn: string, cols: string[]) {
		if (!tab.file || !effectiveDbId) return;
		const prevTableState = tab.file.tables?.[effectiveDbId] ?? {};
		const prevMap = prevTableState.foreignKeyDisplayColumns ?? {};
		updateTab({
			...tab,
			file: {
				...tab.file,
				tables: {
					...(tab.file.tables ?? {}),
					[effectiveDbId]: {
						...prevTableState,
						foreignKeyDisplayColumns: { ...prevMap, [sourceColumn]: cols }
					}
				}
			}
		});
	}

	// Which cell is currently being edited (rowIndex, columnIndex in data columns)
	let editingCell = $state<{ rowIndex: number; columnIndex: number } | null>(null);

	// Measured width of an edited cell's action cluster. Every edited cell has
	// the same expand+rollback pair, so one shared measurement is correct and
	// lets the value box shrink to exactly that width on hover.
	let editActionsWidth = $state(0);

	const totalDataColumns = $derived(queryResult?.columns?.length ?? 0);

	// Live streaming state for the current query result. While streaming, the
	// table derives row count from the watermark; once 'done' arrives,
	// totalRowCount is final.
	const execution = $derived(getExecution(queryResult?.id));
	const isStreaming = $derived(execution?.status === 'streaming');
	const materializedRows = $derived(execution?.available ?? queryResult?.rowCount ?? 0);
	const totalRows = $derived(
		execution?.totalRowCount ?? execution?.available ?? queryResult?.rowCount ?? 0
	);

	function moveEditingCell(direction: 'up' | 'down' | 'left' | 'right') {
		if (!editingCell) return;
		const { rowIndex, columnIndex } = editingCell;
		let nextRow = rowIndex;
		let nextCol = columnIndex;
		if (direction === 'up') nextRow = Math.max(0, rowIndex - 1);
		if (direction === 'down') nextRow = Math.min(totalRows - 1, rowIndex + 1);
		if (direction === 'left') nextCol = Math.max(0, columnIndex - 1);
		if (direction === 'right') nextCol = Math.min(totalDataColumns - 1, columnIndex + 1);
		editingCell = { rowIndex: nextRow, columnIndex: nextCol };
	}

	// Virtual scrolling state
	let virtualScrollState = $state<VirtualScrollState>({
		visibleRange: { start: 0, end: 0 },
		viewportHeight: 0,
		viewportWidth: 0,
		scrollLeft: 0,
		rowHeight: DEFAULT_ROW_HEIGHT,
		scrollContainer: null
	});

	// Column management state
	let columnState = $state<ColumnState>(resetColumnState());

	// Data loading state
	let dataState = $state<DataLoadingState>(createEmptyLoadingState());

	// Composite key (dbId:resultId) that our dataState was built for (null = no data or cleared)
	let dataStateKey = $state<string | null>(null);

	// Derived: width for column i from state, with fallback
	const getColumnWidth = (i: number) => getColumnWidthFn(columnState, i);

	const MIN_COLUMN_WIDTH = 40;

	// Column resize state
	let resizingColumn = $state<number | null>(null);
	let resizeStartX = 0;
	let resizeStartWidth = 0;

	const throttledResize = throttle((e: MouseEvent) => {
		if (resizingColumn === null) return;
		const deltaX = e.clientX - resizeStartX;
		const newW = Math.max(MIN_COLUMN_WIDTH, resizeStartWidth + deltaX);
		const next = columnState.columnWidths.slice();
		if (next.length <= resizingColumn) {
			// Pad with defaults so we can set this column
			while (next.length <= resizingColumn) {
				next.push(resizingColumn === 0 ? 60 : 150);
			}
		}
		next[resizingColumn] = newW;
		columnState = { ...columnState, columnWidths: next };
	}, 16);

	function startResize(columnIndex: number, e: MouseEvent) {
		e.preventDefault();
		e.stopPropagation();
		resizingColumn = columnIndex;
		resizeStartX = e.clientX;
		resizeStartWidth = getColumnWidth(columnIndex);
		document.addEventListener('mousemove', throttledResize);
		document.addEventListener('mouseup', stopResize);
	}

	function stopResize() {
		resizingColumn = null;
		document.removeEventListener('mousemove', throttledResize);
		document.removeEventListener('mouseup', stopResize);

		if (!tab.file || !effectiveDbId) return;
		if (columnState.columnWidths.length === 0) return;

		updateTab({
			...tab,
			file: {
				...tab.file,
				tables: {
					...(tab.file.tables ?? {}),
					[effectiveDbId]: {
						...(tab.file.tables?.[effectiveDbId] ?? {}),
						columnWidths: columnState.columnWidths.slice()
					}
				}
			}
		});
	}

	const visibleRows = $derived.by(() =>
		getVisibleRows(dataState.allRows, virtualScrollState.visibleRange)
	);

	const topSpacerHeight = $derived.by(() =>
		getTopSpacerHeight(virtualScrollState.visibleRange, virtualScrollState.rowHeight)
	);

	const bottomSpacerHeight = $derived.by(() =>
		getBottomSpacerHeight(totalRows, virtualScrollState.visibleRange, virtualScrollState.rowHeight)
	);

	const totalColumns = $derived.by(() => (queryResult?.columns?.length ?? 0) + 1);

	// Horizontal virtualization: visible range of columns
	const visibleColumnRange = $derived.by(() =>
		getVisibleColumnRange(
			columnState,
			totalColumns,
			virtualScrollState.scrollLeft,
			virtualScrollState.viewportWidth || 1
		)
	);

	// Only render: pinned columns + visible range (no DOM for hidden columns)
	const pinnedIndices = $derived.by(() =>
		Array.from(columnState.pinnedColumnIdx).sort((a, b) => a - b)
	);
	const lastPinnedIndex = $derived(pinnedIndices[pinnedIndices.length - 1] ?? -1);

	// Visible non-pinned columns
	const visibleNonPinnedIndices = $derived.by(() => {
		const list: number[] = [];
		for (let i = visibleColumnRange.start; i <= visibleColumnRange.end; i++) {
			if (!columnState.pinnedColumnIdx.has(i)) list.push(i);
		}
		return list;
	});

	// End edit when editing cell scrolls out of view (horizontally or vertically)
	$effect(() => {
		if (!editingCell) return;

		// Check horizontal visibility
		const editColIndex = editingCell.columnIndex + 1;
		const isPinned = columnState.pinnedColumnIdx.has(editColIndex);
		const isColumnVisible = visibleNonPinnedIndices.includes(editColIndex) || isPinned;

		// Check vertical visibility
		const { start, end } = virtualScrollState.visibleRange;
		const isRowVisible = editingCell.rowIndex >= start && editingCell.rowIndex < end;

		if (!isColumnVisible || !isRowVisible) {
			editingCell = null;
		}
	});

	// Spacer widths: sum of hidden non-pinned column widths
	// Left spacer: all non-pinned columns before the first visible non-pinned column
	const leftSpacerWidth = $derived.by(() => {
		let sum = 0;
		for (let i = 0; i < totalColumns; i++) {
			if (columnState.pinnedColumnIdx.has(i)) continue; // skip pinned
			if (i >= visibleColumnRange.start) break; // stop at first visible
			sum += getColumnWidth(i);
		}
		return sum;
	});

	// Right spacer: all non-pinned columns after the last visible non-pinned column
	const rightSpacerWidth = $derived.by(() => {
		let sum = 0;
		for (let i = visibleColumnRange.end + 1; i < totalColumns; i++) {
			if (columnState.pinnedColumnIdx.has(i)) continue; // skip pinned
			sum += getColumnWidth(i);
		}
		return sum;
	});

	// Table width = sum of column widths so table-layout: fixed respects <col> widths
	const tableWidth = $derived.by(() => {
		let sum = 0;
		for (let i = 0; i < totalColumns; i++) sum += getColumnWidth(i);
		return sum;
	});

	// Memoize column styles - compute all at once instead of per-column
	const columnStyles = $derived.by(() => getColumnStylesFn(columnState, totalColumns));

	// Handlers
	const handleRangeChange = async (newRange: VisibleRange) => {
		if (!file) return;

		virtualScrollState.visibleRange = newRange;
		const pageResult = await loadMissingPagesForVisibleRange(
			newRange,
			queryResult,
			effectiveDbId,
			file.id,
			dataState,
			(loadingPages: Set<number>) => {
				dataState.loadingPages = loadingPages;
			},
			totalRows
		);

		if (pageResult) {
			const merged = mergeNewPageIntoState(pageResult, dataState);
			if (merged) dataState = merged;
		}
	};

	const handleVirtualScrollStateChange = (updates: Partial<VirtualScrollState>) => {
		virtualScrollState = { ...virtualScrollState, ...updates };
	};

	const observeViewport = createViewportObserver(
		() => virtualScrollState,
		() => totalRows,
		handleVirtualScrollStateChange,
		handleRangeChange
	);

	const measureRow = createRowMeasurer(
		() => virtualScrollState,
		() => totalRows,
		handleVirtualScrollStateChange,
		handleRangeChange
	);

	const togglePinColumn = (dataColumnIndex: number) => {
		columnState.pinnedColumnIdx = togglePinColumnFn(dataColumnIndex, columnState);
	};

	// Track and save scroll position with debouncing
	// Depends on effectiveDbId so we re-run and get a fresh closure when switching db
	$effect(() => {
		const container = virtualScrollState.scrollContainer;
		const dbId = effectiveDbId;

		if (!container || !dbId) return;

		const saveState = debounce(() => {
			if (!tab.file || !dbId) return;

			updateTab({
				...tab,
				file: {
					...tab.file,
					tables: {
						...(tab.file.tables ?? {}),
						[dbId]: {
							...(tab.file.tables?.[dbId] ?? {}),
							scrollTop: container.scrollTop,
							scrollLeft: container.scrollLeft,
							pinnedColumns: new Set(columnState.pinnedColumnIdx),
							columnWidths:
								columnState.columnWidths.length > 0 ? columnState.columnWidths.slice() : undefined
						}
					}
				}
			});
		}, 100);

		container.addEventListener('scroll', saveState, { passive: true });

		return () => {
			container.removeEventListener('scroll', saveState);
		};
	});

	$effect(() => {
		if (!file) return;
		const fid = file.id;
		const qr = queryResult;
		untrack(() => loadState(fid, qr));
	});

	/**
	 * Load and restore table state from query result
	 * Handles two scenarios:
	 * 1. Pagination: Merge new page into existing result
	 * 2. Initialization: Set up new query or restore from cache when navigating back
	 */
	const loadState = async (fileId: graph.FileNode['id'], newResult: graph.QueryResult | null) => {
		// Clear state if no result
		if (!newResult) {
			dataState = createEmptyLoadingState();
			dataStateKey = null;
			return;
		}

		const dbId = effectiveDbId;
		const currentKey = dbId != null && newResult.id != null ? `${dbId}:${newResult.id}` : null;
		const hasResultData = dataState.allRows.length > 0;
		const shouldInitialize = currentKey !== dataStateKey || !hasResultData;

		// CASE 1: Pagination
		// same query, new page loaded
		if (!shouldInitialize) {
			const mergedState = mergeNewPageIntoState(newResult, dataState);
			if (!mergedState) return;

			dataState = mergedState;
			return;
		}

		// CASE 2: Initialization
		// new query, switched db, or navigating back to file
		dataStateKey = currentKey;
		const cachedState = dbId ? tab.file?.tables?.[dbId] : undefined;

		// Initialize data structures with first page (if any) and the live
		// row total, execution.available while streaming, rowCount once done.
		dataState = createInitialLoadingStateFromResult(newResult, totalRows);
		const totalCols = 1 + (newResult.columns?.length ?? 0);
		const baseColumnState = resetColumnState(totalCols);
		const restoredWidths =
			cachedState?.columnWidths?.length === totalCols
				? cachedState.columnWidths
				: baseColumnState.columnWidths;
		columnState = {
			...baseColumnState,
			pinnedColumnIdx: cachedState?.pinnedColumns ?? baseColumnState.pinnedColumnIdx,
			columnWidths: restoredWidths
		};

		// Pre-load pages needed for cached scroll position
		const { rowHeight, viewportHeight, scrollContainer } = virtualScrollState;
		const targetScrollTop = cachedState?.scrollTop ?? 0;
		const targetScrollLeft = cachedState?.scrollLeft ?? 0;

		const pageResult = await loadMissingPagesForVisibleRange(
			{
				start: Math.floor(targetScrollTop / rowHeight),
				end: Math.floor((targetScrollTop + viewportHeight) / rowHeight)
			},
			newResult,
			effectiveDbId,
			fileId,
			dataState,
			(loadingPages: Set<number>) => {
				dataState.loadingPages = loadingPages;
			},
			totalRows
		);

		if (pageResult) {
			const merged = mergeNewPageIntoState(pageResult, dataState);
			if (merged) dataState = merged;
		}

		// Restore scroll position after DOM updates
		tick().then(() => {
			if (!scrollContainer) return;
			scrollContainer.scrollTop = targetScrollTop;
			scrollContainer.scrollLeft = targetScrollLeft;
		});
	};

	// Update visible range when total row count changes
	$effect(() => {
		const total = totalRows;

		// Only react to result changes, not virtualScrollState changes
		const state = untrack(() => virtualScrollState);

		// Only update if we have a scroll container (table view is mounted)
		if (!state.scrollContainer) return;

		resetVisibleRange(state, total).then((newRange) => {
			if (newRange) {
				untrack(() => {
					virtualScrollState.visibleRange = newRange;
				});
			}
		});
	});

	// While streaming, the watermark advances. Re-run the visible-range loader
	// so partial / pending pages get refetched as rows materialize. Reactive
	// over execution.available; gated on having data and an active execution.
	$effect(() => {
		const available = execution?.available ?? 0;
		const status = execution?.status;
		if (!file) return;
		if (!queryResult || !queryResult.id) return;
		if (status !== 'streaming' && status !== 'done') return;
		if (!virtualScrollState.scrollContainer) return;
		if (available === 0 && status === 'streaming') return;

		const range = untrack(() => virtualScrollState.visibleRange);
		const fid = file.id;
		const qr = queryResult;

		untrack(async () => {
			const pageResult = await loadMissingPagesForVisibleRange(
				range,
				qr,
				effectiveDbId,
				fid,
				dataState,
				(loadingPages: Set<number>) => {
					dataState.loadingPages = loadingPages;
				},
				totalRows
			);
			if (!pageResult) return;
			const merged = mergeNewPageIntoState(pageResult, dataState);
			if (merged) dataState = merged;
		});
	});

	// Utility functions
	const formatCellValue = (value: unknown) => {
		if (value === null) return 'NULL';
		if (value === undefined) return '';
		return typeof value === 'string' ? value : String(value);
	};

	function handleCellClick(e: MouseEvent) {
		const target = e.target as HTMLElement;
		if (!target.classList.contains('text-cell')) return;

		const row = target.dataset.row;
		const col = target.dataset.col;
		if (row === undefined || col === undefined) return;

		const rowIndex = parseInt(row, 10);
		// While streaming, only allow editing on rows that have materialized.
		// Pending rows show a skeleton placeholder; clicks fall through.
		if (isStreaming && rowIndex >= materializedRows) return;

		editingCell = { rowIndex, columnIndex: parseInt(col, 10) };
	}

	function handleCellEdit(rowIndex: number, columnIndex: number, value: string) {
		if (!queryResult || !file) return;
		if (!queryResult.columns) return;

		const columnName = queryResult.columns[columnIndex];
		if (!columnName) return;

		// Check if this column is editable
		const columnMeta = queryResult.columnMetadata?.[columnIndex] || null;
		if (!columnMeta) return;

		// Get original row data to extract PK values
		const originalRow = dataState.allRows[rowIndex];
		if (!originalRow) return;

		// Build primary key values map using column-specific metadata
		const pkValues: Record<string, unknown> = {};
		if (columnMeta.primaryKeys && columnMeta.primaryKeysIdxs) {
			for (let i = 0; i < columnMeta.primaryKeys.length; i++) {
				const pkCol = columnMeta.primaryKeys[i];
				const pkIndex = columnMeta.primaryKeysIdxs[i];
				if (pkIndex !== undefined && originalRow[pkIndex] !== undefined) {
					pkValues[pkCol] = originalRow[pkIndex];
				}
			}
		}

		// Create edit
		const edit: TableEdit = {
			databaseId: columnMeta.databaseId || '',
			schema: columnMeta.schema || '',
			table: columnMeta.table || '',
			column: columnMeta.originalColumnName || columnName,
			value: value,
			rowIndex: rowIndex,
			columnIndex: columnIndex,
			primaryKeyValues: pkValues
		};

		// Update tab state
		const editKey = `${rowIndex}:${columnIndex}`;
		const newEdits = { ...edits };
		const fileState = tab.file!;

		// If value matches original, remove edit
		const originalValue = originalRow[columnIndex];
		if (formatCellValue(originalValue) === value) {
			delete newEdits[editKey];
		} else {
			newEdits[editKey] = edit;
		}

		if (!effectiveDbId) return;

		updateTab({
			...tab,
			file: {
				...fileState,
				tables: {
					...(fileState.tables ?? {}),
					[effectiveDbId]: {
						...(fileState.tables?.[effectiveDbId] ?? {}),
						edits: newEdits
					}
				}
			}
		});
	}

	function isColumnEditable(colIndex: number): boolean {
		const cm = columnMetadataArray[colIndex] || null;
		return (cm?.hasAllPrimaryKeys ?? false) && !(cm?.isPrimaryKey ?? false);
	}

	function openCellModal(rowIndex: number, colIndex: number) {
		if (!queryResult?.columns) return;
		const columnName = queryResult.columns[colIndex];
		const columnMeta = columnMetadataArray[colIndex] || null;
		const edited = editsByRow.get(rowIndex)?.get(colIndex);
		const cell = dataState.allRows[rowIndex]?.[colIndex];
		const fkCtx = foreignKeyContexts[colIndex] || null;

		if (fkCtx) {
			modalStore.set({
				content: () => ForeignKeyPickerModal as unknown as Component,
				width: 'min(80vw, 860px)',
				height: 'min(70vh, 620px)',
				props: {
					databaseId: fkCtx.databaseId,
					currentValue: edited ?? formatCellValue(cell),
					targetSchema: fkCtx.targetSchema,
					targetTable: fkCtx.targetTable,
					targetColumn: fkCtx.targetColumn,
					availableColumns: fkCtx.availableColumns,
					selectedColumns: fkCtx.selectedColumns,
					onSelectColumns: (cols: string[]) =>
						updateForeignKeyDisplayColumns(fkCtx.sourceColumn, cols),
					onSave: (v: string) => handleCellEdit(rowIndex, colIndex, v)
				}
			});
			return;
		}

		modalStore.set({
			content: () => CellValueModal as unknown as Component,
			width: 'min(80vw, 860px)',
			height: 'min(70vh, 620px)',
			props: {
				value: edited ?? formatCellValue(cell),
				dataType: columnMeta?.dataType,
				columnName,
				onSave: (v: string) => handleCellEdit(rowIndex, colIndex, v)
			}
		});
	}

	function handleRollbackEdit(rowIndex: number, colIndex: number) {
		const editKey = `${rowIndex}:${colIndex}`;
		const newEdits = { ...edits };
		delete newEdits[editKey];

		if (!tab.file || !effectiveDbId) return;

		updateTab({
			...tab,
			file: {
				...tab.file,
				tables: {
					...(tab.file.tables ?? {}),
					[effectiveDbId]: {
						...(tab.file.tables?.[effectiveDbId] ?? {}),
						edits: newEdits
					}
				}
			}
		});
	}
</script>

{#snippet headerCell(columnIndex: number, isPinned: boolean)}
	{#if columnIndex === 0}
		<span class="cell" style="display: block; text-align: end;">#</span>
	{:else}
		{@const colNode = columnNodes[columnIndex - 1]}
		<Group>
			<!-- svelte-ignore a11y_click_events_have_key_events -->
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<span
				class="cell"
				class:clickable={!!colNode}
				onclick={colNode
					? () =>
							modalStore.set({ content: () => ItemInfoModal, props: { item: colNode }, width: 600 })
					: undefined}>{queryResult?.columns?.[columnIndex - 1]}</span
			>
			<Button
				onclick={() => togglePinColumn(columnIndex - 1)}
				emphasis={isPinned ? 'high' : 'low'}
				leftIcon={isPinned ? 'pinned' : 'pin'}
				iconSize={isPinned ? 15 : 16}
				size="sm"
			/>
		</Group>
	{/if}
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<span
		class="col-resizer"
		class:active={resizingColumn === columnIndex}
		role="separator"
		aria-orientation="vertical"
		onmousedown={(e) => startResize(columnIndex, e)}
	></span>
{/snippet}

{#snippet dataCell(
	columnIndex: number,
	rowIndex: number,
	row: unknown[] | null,
	rowEdits: Map<number, string> | undefined,
	isEditingThisRow: boolean
)}
	{#if columnIndex === 0}
		<span class="text-cell" style="display: block; text-align: end;">{rowIndex + 1}</span>
	{:else if row}
		{@const colIndex = columnIndex - 1}
		{@const cell = row[colIndex]}
		{#if isEditingThisRow && editingCell?.columnIndex === colIndex}
			{@const columnMeta = columnMetadataArray[colIndex] || null}
			{@const editedValue = rowEdits?.get(colIndex)}
			{@const fkCtx = foreignKeyContexts[colIndex] || null}
			<EditableCell
				isEdited={editedValue !== undefined}
				value={editedValue ?? formatCellValue(cell)}
				hasAllPrimaryKeys={columnMeta?.hasAllPrimaryKeys ?? false}
				isPrimaryKey={columnMeta?.isPrimaryKey ?? false}
				enumValues={columnMeta?.enumValues}
				dataType={columnMeta?.dataType}
				columnName={queryResult?.columns?.[colIndex]}
				missingPrimaryKeys={missingPrimaryKeysPerColumn[colIndex] ?? []}
				foreignKey={fkCtx
					? ({
							databaseId: fkCtx.databaseId,
							targetSchema: fkCtx.targetSchema,
							targetTable: fkCtx.targetTable,
							targetColumn: fkCtx.targetColumn,
							availableColumns: fkCtx.availableColumns,
							selectedColumns: fkCtx.selectedColumns,
							onSelectColumns: (cols: string[]) =>
								updateForeignKeyDisplayColumns(fkCtx.sourceColumn, cols)
						} satisfies ForeignKeyContext)
					: undefined}
				onEdit={(value: string) => handleCellEdit(rowIndex, colIndex, value)}
				onEndEdit={() => (editingCell = null)}
				onMoveEdit={moveEditingCell}
				onRollback={() => handleRollbackEdit(rowIndex, colIndex)}
			/>
		{:else}
			{@const editedValue = rowEdits?.get(colIndex)}
			{#if editedValue !== undefined}
				<div class="edited-cell-wrapper" style="--actions-w: {editActionsWidth}px;">
					<span class="text-cell edited" data-row={rowIndex} data-col={colIndex}>{editedValue}</span
					>
					<div class="cell-actions" bind:clientWidth={editActionsWidth}>
						{#if isColumnEditable(colIndex)}
							<Button
								onclick={() => openCellModal(rowIndex, colIndex)}
								label="Open in editor"
								leftIcon="arrow-diagonal"
								iconSize={16}
								size="sm"
								emphasis="low"
							/>
						{/if}
						<Button
							onclick={() => handleRollbackEdit(rowIndex, colIndex)}
							label="Rollback edit"
							leftIcon="refresh"
							iconSize={16}
							size="sm"
							emphasis="low"
						/>
					</div>
				</div>
			{:else}
				<span class="text-cell" data-row={rowIndex} data-col={colIndex}
					>{formatCellValue(cell)}</span
				>
			{/if}
		{/if}
	{:else}
		<span class="cell loading-cell">...</span>
	{/if}
{/snippet}

<div class="wrapper selectable" class:resizing={resizingColumn !== null}>
	<!-- Columns are enough to render: an empty result set still shows its headers. -->
	{#if queryResult?.columns?.length}
		{@const colSpan =
			pinnedIndices.length +
			visibleNonPinnedIndices.length +
			(leftSpacerWidth > 0 ? 1 : 0) +
			(rightSpacerWidth > 0 ? 1 : 0)}
		<div class="table scrollable" use:observeViewport data-total-rows={totalRows}>
			<table border="1" cellpadding="5" cellspacing="0" style="width: {tableWidth}px;">
				<colgroup>
					{#each pinnedIndices as columnIndex (columnIndex)}
						<col style="width: {getColumnWidth(columnIndex)}px;" />
					{/each}
					{#if leftSpacerWidth > 0}<col style="width: {leftSpacerWidth}px;" />{/if}
					{#each visibleNonPinnedIndices as columnIndex (columnIndex)}
						<col style="width: {getColumnWidth(columnIndex)}px;" />
					{/each}
					{#if rightSpacerWidth > 0}<col style="width: {rightSpacerWidth}px;" />{/if}
				</colgroup>
				<thead>
					<tr>
						{#each pinnedIndices as columnIndex (columnIndex)}
							<th
								class="sticky"
								class:last-pinned-column={columnIndex === lastPinnedIndex}
								class:active={resizingColumn === columnIndex}
								style={`top: 0; ${columnStyles.get(columnIndex)}`}
							>
								{@render headerCell(columnIndex, true)}
							</th>
						{/each}
						{#if leftSpacerWidth > 0}<th class="spacer-cell"></th>{/if}
						{#each visibleNonPinnedIndices as columnIndex (columnIndex)}
							<th class:active={resizingColumn === columnIndex} style="top: 0;">
								{@render headerCell(columnIndex, false)}
							</th>
						{/each}
						{#if rightSpacerWidth > 0}<th class="spacer-cell"></th>{/if}
					</tr>
				</thead>
				<!-- svelte-ignore a11y_click_events_have_key_events -->
				<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
				<tbody onclick={handleCellClick}>
					{#if topSpacerHeight > 0}
						<tr class="spacer top">
							<td colspan={colSpan} style={`height: ${topSpacerHeight}px;`}></td>
						</tr>
					{/if}
					{#each visibleRows as row, localIndex (virtualScrollState.visibleRange.start + localIndex)}
						{@const rowIndex = virtualScrollState.visibleRange.start + localIndex}
						{@const rowEdits = editsByRow.get(rowIndex)}
						{@const isEditingThisRow = editingCell?.rowIndex === rowIndex}
						<tr
							use:measureRow={rowIndex === virtualScrollState.visibleRange.start}
							class:loading-row={!row}
						>
							{#each pinnedIndices as columnIndex (columnIndex)}
								<td
									class="sticky"
									class:last-pinned-column={columnIndex === lastPinnedIndex}
									style={columnStyles.get(columnIndex)}
								>
									{@render dataCell(columnIndex, rowIndex, row, rowEdits, isEditingThisRow)}
								</td>
							{/each}

							{#if leftSpacerWidth > 0}<td class="spacer-cell"></td>{/if}

							{#each visibleNonPinnedIndices as columnIndex (columnIndex)}
								<td>
									{@render dataCell(columnIndex, rowIndex, row, rowEdits, isEditingThisRow)}
								</td>
							{/each}
							{#if rightSpacerWidth > 0}<td class="spacer-cell"></td>{/if}
						</tr>
					{/each}
					{#if bottomSpacerHeight > 0}
						<tr class="spacer bottom">
							<td colspan={colSpan} style={`height: ${bottomSpacerHeight}px;`}></td>
						</tr>
					{/if}
				</tbody>
			</table>
		</div>
	{/if}
</div>

<style>
	.wrapper {
		height: 100%;
		overflow: hidden;
		background-color: var(--gray-200);
	}
	.wrapper.resizing {
		cursor: col-resize;
		user-select: none;
	}

	.table {
		overflow: auto;
		height: 100%;
		will-change: scroll-position;
		contain: strict;
	}

	.cell,
	.text-cell {
		color: var(--gray-800);
	}

	th .cell {
		color: var(--gray-800);
		font-weight: var(--fw-light);
	}

	th .cell.clickable:hover {
		color: var(--gray-1000);
	}

	table {
		border-collapse: collapse;
		table-layout: fixed !important;
		border-top: none;
		border-right: var(--border);
		border-bottom: none;
		border-left: none;
	}

	th {
		position: sticky;
		top: 0;
		z-index: 1;
	}

	tbody {
		transform: translateZ(0); /* Force GPU compositing for smoother scroll */
	}

	tbody tr {
		/* Avoid contain: layout so rows reflow when column widths / sticky left change (e.g. resize pinned column) */
		contain: paint;
	}

	td {
		height: 24px;
		max-height: 24px;
		overflow: hidden;
		/* content-visibility removed so cell layout updates when column widths change (resize pinned column) */
		contain-intrinsic-size: auto 24px;
	}

	th {
		overflow: hidden;
		background-color: var(--gray-200);
	}

	th span:not(.col-resizer) {
		padding: var(--space-sm-md) var(--space-sm);
	}

	th,
	td {
		text-align: left;
		border-top: none;
		border-right: none;
		border-bottom: none;
		border-left: none;
	}

	th span,
	td span {
		white-space: nowrap;
		text-overflow: ellipsis;
		overflow: hidden;
	}

	tr:hover td .cell,
	tr:hover td .text-cell {
		color: var(--gray-1000);
	}

	tr:hover td {
		background-color: var(--gray-0);
	}

	tr:first-child td,
	tr:first-child th {
		border-top: none;
	}

	tr:first-child th,
	tr:last-child th {
		border-bottom: none;
	}

	tr td:first-child,
	tr th:first-child {
		border-left: none;
	}

	tr td:last-child,
	tr th:last-child {
		border-right: none;
	}

	/* Spacer cells for horizontal virtualization */
	.spacer-cell {
		padding: 0;
		border: none;
		background: var(--gray-100);
	}
	th.spacer-cell {
		background: var(--gray-200);
	}

	/* Last pinned column: right border to separate from scrollable area (override base border-right: none) */
	:global(th.last-pinned-column > *:not(.col-resizer)),
	:global(td.last-pinned-column > span:first-child) {
		border-right: var(--border) !important;
	}

	/* Pinned columns behaviors */
	th.sticky {
		z-index: 3 !important;
		background: var(--gray-200);
	}
	td.sticky {
		position: sticky;
		background: var(--gray-200);
		z-index: 2;
	}
	th.sticky,
	td.sticky {
		/* Force GPU compositing to prevent flicker during scroll */
		transform: translateZ(0);
		backface-visibility: hidden;
		contain: layout style paint;
	}

	/* Column resizer - positioned at right edge of each th (sticky creates containing block) */
	.col-resizer {
		position: absolute;
		top: 0;
		right: 0;
		bottom: 0;
		width: 3px;
		border-right: 0.5px var(--gray-700) solid;
		cursor: col-resize;
		user-select: none;
	}
	.col-resizer:hover,
	.col-resizer.active {
		background: var(--gray-700);
	}
	.col-resizer.active {
		background: var(--gray-700);
	}

	/* Column actions */
	:global(th button),
	th .col-resizer {
		opacity: 0;
		transition: opacity 0.2s ease-out;
	}
	:global(th:hover button),
	:global(th.sticky button),
	:global(th.active button),
	th.active .col-resizer,
	th:hover .col-resizer {
		opacity: 1;
	}

	tr.spacer td {
		padding: 0;
		border: none;
		height: auto;
	}
	tr.spacer {
		pointer-events: none;
		background: transparent;
	}

	/* Loading row styles */
	tr.loading-row {
		opacity: 0.5;
	}

	.loading-cell {
		color: var(--gray-800);
		font-style: italic;
	}

	td > .text-cell {
		display: block;
		padding: var(--space-sm-md) var(--space-sm);
	}

	/* Edited cell wrapper and styles */
	.edited-cell-wrapper {
		position: relative;
		display: block;
		height: 100%;
	}
	.edited-cell-wrapper > .text-cell.edited {
		display: block;
		padding: var(--space-sm-md) var(--space-sm);
		/* always reserve room for the expand+rollback buttons */
		padding-right: calc(var(--space-sm) + var(--space-xs) + var(--actions-w, 0px));
		width: 100%;
		height: 100%;
		box-sizing: border-box;
		background-color: var(--gray-300);
		color: var(--gray-1000);

		border-radius: var(--br-sm);
		outline: 0.5px solid var(--green);
		outline-offset: -1px;

		box-shadow:
			var(--shadow) 0px 10px 5px -2px inset,
			var(--shadow) 0px 5px 5px -5px inset;
	}

	.cell-actions {
		position: absolute;
		right: var(--space-sm);
		top: 50%;
		transform: translateY(-50%);
		display: flex;
		align-items: center;
		gap: var(--space-xs);
		z-index: 1;
		opacity: 0;
		transition: opacity 0.15s ease-out;
	}
	.edited-cell-wrapper:hover .cell-actions {
		opacity: 1;
	}
</style>
