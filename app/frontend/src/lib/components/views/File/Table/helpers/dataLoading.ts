import { GetResultPage } from '$lib/wailsjs/go/db_client/DbClient';
import { must, tryCatch } from '$lib/utils/tryCatch';
import type { graph } from '$lib/wailsjs/go/models';
import type { VisibleRange } from './virtualScrolling';

export interface DataLoadingState {
	allRows: unknown[][];
	loadedPages: Set<number>;
	// partialPages tracks pages where some rows arrived but more are streaming.
	// They count as "loaded" for the rows they hold, but stay refetchable.
	partialPages: Set<number>;
	loadingPages: Set<number>;
}

export function calculatePageNumber(rowIndex: number, pageSize: number): number {
	return Math.floor(rowIndex / pageSize);
}

export function isPageAlreadyLoadedOrLoading(
	loadingState: DataLoadingState,
	pageNumber: number
): boolean {
	return loadingState.loadedPages.has(pageNumber) || loadingState.loadingPages.has(pageNumber);
}

// isPageRefetchable returns true when a page should be re-requested because
// only a partial slice has arrived so far. Used to drive watermark-driven
// refetches while a query is streaming.
export function isPageRefetchable(loadingState: DataLoadingState, pageNumber: number): boolean {
	if (loadingState.loadingPages.has(pageNumber)) return false;
	if (loadingState.loadedPages.has(pageNumber)) return false;
	return loadingState.partialPages.has(pageNumber);
}

export async function requestPageFromBackend(
	pageNumber: number,
	queryResult: graph.QueryResult | null,
	dbInstanceId: string | null | undefined,
	fileId: string | undefined,
	currentLoadingState: DataLoadingState,
	onLoadingPagesChange: (loadingPages: Set<number>) => void
): Promise<graph.QueryResult | null> {
	if (
		!queryResult?.id ||
		!dbInstanceId ||
		!fileId ||
		pageNumber < 0 ||
		currentLoadingState.loadedPages.has(pageNumber) ||
		currentLoadingState.loadingPages.has(pageNumber)
	) {
		return null;
	}

	const updatedLoadingPages = new Set([...currentLoadingState.loadingPages, pageNumber]);
	onLoadingPagesChange(updatedLoadingPages);

	const result = await must(
		tryCatch(GetResultPage, {
			DbInstanceID: dbInstanceId,
			FileID: fileId,
			ResultID: queryResult.id,
			Page: pageNumber
		}),
		null,
		() => {
			const cleanedLoadingPages = new Set(
				[...currentLoadingState.loadingPages].filter((p) => p !== pageNumber)
			);
			onLoadingPagesChange(cleanedLoadingPages);
		}
	);

	return result;
}

export async function loadMissingPagesForVisibleRange(
	visibleRange: VisibleRange,
	queryResult: graph.QueryResult | null,
	dbInstanceId: string | null | undefined,
	fileId: string | undefined,
	currentLoadingState: DataLoadingState,
	onLoadingPagesChange: (loadingPages: Set<number>) => void,
	totalRows: number
): Promise<graph.QueryResult | null> {
	if (!queryResult) return null;

	const pageSize = queryResult.pageSize || 75;
	const firstPageInRange = calculatePageNumber(visibleRange.start, pageSize);
	const lastPageInRange = calculatePageNumber(Math.max(visibleRange.end - 1, visibleRange.start), pageSize);
	const maxPage = totalRows > 0 ? Math.ceil(totalRows / pageSize) - 1 : -1;

	const candidatePages: number[] = [];
	for (let pageNum = firstPageInRange; pageNum <= lastPageInRange; pageNum++) {
		if (maxPage >= 0 && pageNum > maxPage) continue;
		if (isPageAlreadyLoadedOrLoading(currentLoadingState, pageNum)) continue;
		candidatePages.push(pageNum);
	}

	const middlePageNumber = Math.floor((firstPageInRange + lastPageInRange) / 2);
	candidatePages.sort((a, b) => Math.abs(a - middlePageNumber) - Math.abs(b - middlePageNumber));

	if (candidatePages.length < 1) return null;

	return await requestPageFromBackend(
		candidatePages[0],
		queryResult,
		dbInstanceId,
		fileId,
		currentLoadingState,
		onLoadingPagesChange
	);
}

export function createInitialLoadingStateFromResult(
	queryResult: graph.QueryResult,
	totalRowCount: number
): DataLoadingState {
	const preallocatedRowsArray = new Array(Math.max(totalRowCount, 0));
	const initialLoadedPages = new Set<number>();
	const initialPartialPages = new Set<number>();

	// Insert the initial page data that came with the query result, if any.
	if (queryResult.rows && queryResult.rows.length > 0) {
		const pageSize = queryResult.pageSize || 75;
		const initialPageNumber = queryResult.page ?? 0;
		const startRowIndex = initialPageNumber * pageSize;

		for (let i = 0; i < queryResult.rows.length; i++) {
			preallocatedRowsArray[startRowIndex + i] = queryResult.rows[i];
		}

		if (queryResult.status === 'partial') {
			initialPartialPages.add(initialPageNumber);
		} else {
			initialLoadedPages.add(initialPageNumber);
		}
	}

	return {
		allRows: preallocatedRowsArray,
		loadedPages: initialLoadedPages,
		partialPages: initialPartialPages,
		loadingPages: new Set()
	};
}

export function mergeNewPageIntoState(
	newPageResult: graph.QueryResult,
	currentLoadingState: DataLoadingState
): DataLoadingState | null {
	if (newPageResult.page === undefined) return null;

	const status = newPageResult.status ?? 'ready';
	const incomingRows = newPageResult.rows ?? [];

	// 'pending' means the page lies past the watermark; nothing to merge yet.
	// Clear it from loadingPages so the next watermark tick will refetch.
	if (status === 'pending') {
		const nextLoading = new Set(currentLoadingState.loadingPages);
		nextLoading.delete(newPageResult.page);
		if (nextLoading.size === currentLoadingState.loadingPages.size) return null;
		return { ...currentLoadingState, loadingPages: nextLoading };
	}

	if (incomingRows.length === 0 && status === 'ready') {
		// Empty terminal page (past end of finished stream); mark as loaded so
		// we don't keep retrying.
		const nextLoaded = new Set(currentLoadingState.loadedPages);
		nextLoaded.add(newPageResult.page);
		const nextLoading = new Set(currentLoadingState.loadingPages);
		nextLoading.delete(newPageResult.page);
		const nextPartial = new Set(currentLoadingState.partialPages);
		nextPartial.delete(newPageResult.page);
		return {
			...currentLoadingState,
			loadedPages: nextLoaded,
			partialPages: nextPartial,
			loadingPages: nextLoading
		};
	}

	const pageSize = newPageResult.pageSize || 75;
	const startRowIndex = newPageResult.page * pageSize;

	// Grow the buffer as new rows materialize past the previous watermark.
	const requiredLength = startRowIndex + incomingRows.length;
	const updatedRowsArray =
		requiredLength > currentLoadingState.allRows.length
			? currentLoadingState.allRows.concat(
					new Array(requiredLength - currentLoadingState.allRows.length)
				)
			: [...currentLoadingState.allRows];
	for (let i = 0; i < incomingRows.length; i++) {
		updatedRowsArray[startRowIndex + i] = incomingRows[i];
	}

	const nextLoaded = new Set(currentLoadingState.loadedPages);
	const nextPartial = new Set(currentLoadingState.partialPages);
	const nextLoading = new Set(currentLoadingState.loadingPages);
	nextLoading.delete(newPageResult.page);

	if (status === 'partial') {
		nextPartial.add(newPageResult.page);
		nextLoaded.delete(newPageResult.page);
	} else {
		nextLoaded.add(newPageResult.page);
		nextPartial.delete(newPageResult.page);
	}

	return {
		allRows: updatedRowsArray,
		loadedPages: nextLoaded,
		partialPages: nextPartial,
		loadingPages: nextLoading
	};
}

export function createEmptyLoadingState(): DataLoadingState {
	return {
		allRows: [],
		loadedPages: new Set(),
		partialPages: new Set(),
		loadingPages: new Set()
	};
}
