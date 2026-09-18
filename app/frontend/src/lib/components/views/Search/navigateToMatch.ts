import { addTab } from '$lib/components/Layout/layoutStore';
import * as graph from '$lib/wails/graph';
import { EventsEmit } from '$lib/wails/events';

/**
 * Parses a search match URI to extract the file URI, line number, and column.
 * Search match URIs have the format: selectdb://workspaces/{workspaceId}/{path}::L{line}::C{column}
 */
const parseSearchMatchUri = (uri: string): { fileUri: string; line?: number; column?: number } => {
	const lineIndex = uri.indexOf('::L');
	if (lineIndex === -1) return { fileUri: uri };

	const fileUri = uri.substring(0, lineIndex);
	const fragment = uri.substring(lineIndex + 3); // Skip "::L"

	// Parse line and column from fragment (e.g., "10::C5" -> line=10, column=5)
	const colIndex = fragment.indexOf('::C');
	if (colIndex === -1) {
		const line = parseInt(fragment, 10);
		return { fileUri, line: isNaN(line) ? undefined : line };
	}

	const line = parseInt(fragment.substring(0, colIndex), 10);
	const column = parseInt(fragment.substring(colIndex + 3), 10); // Skip "::C"

	return {
		fileUri,
		line: isNaN(line) ? undefined : line,
		column: isNaN(column) ? undefined : column
	};
};

/**
 * Navigates to a specific line and column in a file (search match).
 * This opens the file in an editor tab and scrolls to the specified position.
 */
export const navigateToMatch = async (searchMatchNode: graph.FileNode) => {
	const { fileUri, line, column } = parseSearchMatchUri(searchMatchNode.uri);

	addTab(
		graph.newFileNode({
			...searchMatchNode,
			id: fileUri,
			uri: fileUri,
			name: fileUri.split('/').pop() || 'unknown'
		})
	);

	if (line !== undefined) {
		setTimeout(() => {
			EventsEmit('editor:scrollToLine', { uri: fileUri, line, column: column ?? 1 });
		}, 100);
	}
};
