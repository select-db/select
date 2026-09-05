import { WorkspaceURIPrefix } from '$lib/bindings/selectDb/internal/fs_provider/fsprovider';

/**
 * Whether an id names a file or folder on disk, asking the Go side that defines
 * the prefix rather than restating it here.
 *
 * The trees render more than the workspace: a search result ("search::file::…")
 * and a git status row ("git::staged") carry ids that name neither.
 */
export async function isWorkspaceUri(id: string): Promise<boolean> {
	return id.startsWith(await WorkspaceURIPrefix());
}
