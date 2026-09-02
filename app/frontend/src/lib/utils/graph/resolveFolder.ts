import { ResolveFolder } from '$lib/wails/graph';

/**
 * A workspace graph carries the folders of a workspace but only the files of
 * the folders that have been opened — the backend reads a folder's files the
 * first time it is asked for them, and pushes an updated graph when it does.
 *
 * So opening a folder in the tree asks for it. This is fire-and-forget: the
 * files arrive with the next graph update, the same way any other change to the
 * workspace does.
 *
 * Only nodes addressed by a workspace URI are real folders on disk. The search
 * and git panels render trees of their own whose ids ("search::file::…",
 * "git::staged") name results, not directories, and they are left alone.
 */
const WORKSPACE_URI_PREFIX = 'selectdb://workspaces/';

const asked = new Set<string>();

export function resolveFolderContents(id: string) {
	if (!id.startsWith(WORKSPACE_URI_PREFIX)) return;
	if (asked.has(id)) return;

	asked.add(id);
	void ResolveFolder(id).catch(() => {
		// Let the next open try again rather than leaving the folder stuck
		// empty on a transient failure.
		asked.delete(id);
	});
}

/** Called when the graph is dropped, so a reloaded workspace asks again. */
export function clearResolvedFolders() {
	asked.clear();
}
