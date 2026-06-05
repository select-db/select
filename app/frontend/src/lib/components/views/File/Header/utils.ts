export const getPathFromUri = (uri: string) => {
	const parts = uri.split('/').filter(Boolean);
	const workspacesIdx = parts.indexOf('workspaces');
	if (workspacesIdx === -1) return parts;
	return parts.slice(workspacesIdx + 2);
};
