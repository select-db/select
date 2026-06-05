import { must, tryCatch } from '$lib/utils/tryCatch';
import * as fs from '$lib/wailsjs/go/fs_provider/FSProvider';

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

export const writeDatabase = async (
	parentUri: string,
	displayName: string
): Promise<{ id: string; uri: string }> => {
	// Generate stable folder name using ID-based format
	const id = crypto.randomUUID();
	const stableFolderName = `db-${id}`;
	const dbUri = `${parentUri}/${stableFolderName}`;

	// Create the folder first
	await must(
		tryCatch(fs.Mkdir, {
			uri: dbUri
		})
	);

	// Write db.config.json with proper content including the display name
	// The filesystem watcher will detect this and create the DB instance node
	const config = {
		id,
		name: displayName,
		db_type: 'postgresql',
		dsn: ''
	};

	await must(
		tryCatch(fs.Write, {
			uri: `${dbUri}/db.config.json`,
			content: JSON.stringify(config, null, 2)
		})
	);

	return { id, uri: dbUri };
};
