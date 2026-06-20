import type { graph, system } from '$lib/wailsjs/go/models';
import { GetUserConfigResources } from '$lib/wailsjs/go/system/System';
import { tryCatch } from '$lib/utils/tryCatch';
import type { ResourceMenuOption } from './types';

/**
 * Personal config files (.theme, .config keybindings/snippets) live in the
 * per-user config dir, outside the workspace graph. We still surface them in the
 * global resource menu and open them in the regular file editing experience by
 * wrapping each one in a synthetic FileNode keyed on its selectdb://user/... URI.
 *
 * The node name is the real file name (".theme" / ".config") so file.svelte
 * routes it to the right editor view, and BaseFileView reads/writes its content
 * through the FSProvider using the user URI.
 */
function userFileNode(resource: system.UserConfigResource): graph.FileNode {
	return {
		id: resource.uri,
		name: resource.name,
		type: 'file',
		path: '',
		uri: resource.uri,
		folder_id: '',
		badges: [],
		convertValues: () => ({})
	} as unknown as graph.FileNode;
}

function toOption(resource: system.UserConfigResource): ResourceMenuOption {
	return {
		id: resource.uri,
		label: resource.name,
		type: 'file',
		uri: resource.uri,
		node: userFileNode(resource)
	};
}

/**
 * Loads the per-user config files from the backend and maps them to resource
 * menu options. Returns an empty list on error so search never breaks.
 */
export async function loadUserConfigResourceOptions(): Promise<ResourceMenuOption[]> {
	const [resources, err] = await tryCatch(GetUserConfigResources);
	if (err || !resources) return [];
	return resources.map(toOption);
}
