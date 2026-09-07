import { must, tryCatch } from '$lib/utils/tryCatch';
import * as fs from '$lib/bindings/selectDb/internal/fs_provider/fsprovider';
import * as graph from '$lib/bindings/selectDb/internal/graph/graph';
import type * as models from '$lib/bindings/selectDb/internal/graph/models';

export const writeFolder = async (uri: string) => {
	await must(
		tryCatch(fs.Mkdir, {
			uri
		})
	);
};

export const writeFile = async (uri: string) => {
	await must(
		tryCatch(fs.Write, {
			uri,
			content: ''
		})
	);

	// Write .metadata.json sidecar with proper content
	// The filesystem watcher will detect this and mutate the new file
	const config = { databases: [] };

	await must(
		tryCatch(fs.Write, {
			uri: uri + '.metadata.json',
			content: JSON.stringify(config, null, 2)
		})
	);
};

export const createDatabase = async (
	parentUri: string,
	name: string
): Promise<models.CreatedDatabase> => {
	// The directory and its db.config.json are written together in the backend:
	// picking the name means seeing what the folder already holds, and a
	// database that exists as a directory but not yet as a config is a folder.
	const created = await must(
		tryCatch(graph.CreateDatabase, {
			folder_uri: parentUri,
			name
		})
	);
	if (!created) throw new Error(`Could not create ${name}`);
	return created;
};
