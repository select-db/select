import * as graph from '$lib/wails/graph';
import type * as git from '$lib/bindings/selectDb/internal/git/models';

/**
 * Creates a synthetic FileNode from a git file path.
 */
function createSyntheticFileNode(
	workspaceId: string,
	path: string,
	status: string,
	porcelainCode: string
): graph.FileNode {
	const parts = path.split('/');
	const name = parts[parts.length - 1];

	return graph.newFileNode({
		// We add git::<status> in file.id to help identify status
		// in the fs getActions, getOptions
		id: `git::${status}/${path}`,
		uri: gitPathToURI(workspaceId, path),
		type: 'file',
		name,
		badges: [porcelainCode]
	});
}

export function mapGitFilesToNodes(
	gitFiles: git.GitFileStatusItem[],
	workspaceId: string
): graph.FileNode[] {
	return gitFiles.map(({ path, porcelainCode, status }) => {
		return createSyntheticFileNode(workspaceId, path, status, porcelainCode);
	});
}

const fsPrefix = 'selectdb://workspaces';
/**
 * Extracts the relative git path from a FileNode URI.
 * Converts "selectdb://workspaces/<workspaceId>/git://<status>/path/to/file" to "path/to/file"
 */
export const uriToGitPath = (uri: string, workspaceId: string): string => {
	const prefix = `${fsPrefix}/${workspaceId}/`;
	return uri.slice(prefix.length);
};

/**
 * Converts a git file path to a selectdb:// URI format.
 * Git paths are relative to workspace root, e.g., "folder/file.sql"
 * URIs are in format: "selectdb://workspaces/<workspaceId>/folder/file.sql"
 */
function gitPathToURI(workspaceId: string, gitPath: string): string {
	// Normalize path separators and ensure it starts with workspace prefix
	const normalizedPath = gitPath.replace(/\\/g, '/');
	return `${fsPrefix}/${workspaceId}/${normalizedPath}`;
}
