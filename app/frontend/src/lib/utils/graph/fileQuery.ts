import { FindFiles, type FileNode } from '$lib/wails/graph';

/**
 * The files a picker should show for what has been typed.
 *
 * The workspace graph holds the files of the folders that have been opened —
 * what the tree shows — so it cannot answer for the rest of the workspace. The
 * backend can, and it answers a question rather than handing over a listing: a
 * pattern, capped. Each keystroke asks again and cancels the answer it no
 * longer wants, so a large workspace costs one interrupted walk rather than a
 * copy of itself in memory.
 *
 * An empty pattern asks nothing. What to show before anything is typed —
 * recently opened files, open tabs — is the picker's business, not a query's.
 */
const RESULT_LIMIT = 200;

export type FileQueryHandle = { cancel: () => void };

export function queryWorkspaceFiles(
	pattern: string,
	onResults: (files: FileNode[]) => void
): FileQueryHandle {
	const trimmed = pattern.trim();
	if (!trimmed) {
		onResults([]);
		return { cancel: () => {} };
	}

	const request = FindFiles({ pattern: trimmed, limit: RESULT_LIMIT }).then(
		(files) => onResults(files),
		() => {
			// A cancelled or failed query leaves the last results in place rather
			// than blanking the menu under the cursor.
		}
	);

	return { cancel: () => request.cancel() };
}
