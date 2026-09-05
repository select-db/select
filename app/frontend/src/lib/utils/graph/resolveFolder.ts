import { ResolveFolder } from '$lib/wails/graph';
import { isWorkspaceUri } from '$lib/utils/graph/workspaceUri';

// The graph carries every folder but only the files of the ones that have been
// opened, so opening one asks the backend to read it. Fire-and-forget: the files
// arrive with the next graph update.

/**
 * Folders with a read in flight, so a double click asks once.
 *
 * Deliberately not a memory of what has already been read: a rebuild — after a
 * rename, a branch switch — hands back a folder that has to be read again, and
 * a folder the backend has already read is answered without touching the disk.
 */
const inFlight = new Set<string>();

export async function resolveFolderContents(id: string) {
	// Ids from the search and git trees name results rather than directories.
	if (!(await isWorkspaceUri(id)) || inFlight.has(id)) return;

	inFlight.add(id);
	const done = () => inFlight.delete(id);
	void ResolveFolder(id).then(done, done);
}
