import { must, tryCatch } from '$lib/utils/tryCatch';
import * as fs from '$lib/bindings/selectDb/internal/fs_provider/fsprovider';

/** The file that makes a directory a database. */
const DB_CONFIG_FILE = 'db.config.json';

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
	name: string
): Promise<{ id: string; uri: string }> => {
	const id = crypto.randomUUID();
	const dbUri = `${parentUri}/${name}`;

	await must(tryCatch(fs.Mkdir, { uri: dbUri }));

	// The directory is only a folder until this lands: the watcher reads the
	// config and turns it into a db instance node.
	const config = {
		id,
		name,
		db_type: 'postgresql',
		dsn: ''
	};

	await must(
		tryCatch(fs.Write, {
			uri: `${dbUri}/${DB_CONFIG_FILE}`,
			content: JSON.stringify(config, null, 2)
		})
	);

	return { id, uri: dbUri };
};
