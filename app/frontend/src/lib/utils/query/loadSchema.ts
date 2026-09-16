import * as graph from '$lib/wails/graph';
import { QuerySchema } from '$lib/bindings/selectDb/internal/db_client/dbclient';

import { AlertType } from '$lib/system/Alert/types';
import { notify, notifyError } from '$lib/system/Notifications/notificationsStore';
import { pushToLoadingStore, removeFromLoadingStore } from './loadingStore';
import { tryCatch } from '../tryCatch';

/**
 * Loads a database's schema, reporting failure the way the rest of the app
 * does. A database that cannot be read says so rather than expanding to
 * nothing with the reason in the log.
 */
export const loadSchema = async ({
	database,
	announce = true
}: {
	database: graph.DBInstanceNode;
	/** Says "schema loaded" when it worked. Off for a read nobody asked for. */
	announce?: boolean;
}) => {
	pushToLoadingStore(database.id);

	// Past the cache always: a caller here is either a person asking again or a
	// database showing nothing, and the cached answer is what both are behind.
	const [, err] = await tryCatch(QuerySchema, {
		DatabaseInstanceID: database.id,
		NoCache: true
	});

	removeFromLoadingStore(database.id);

	// Reports and stops: announcing "schema loaded" straight after an error is
	// what made this read as a silent failure.
	if (err) {
		notifyError(err.message);
		return;
	}

	if (announce)
		notify({
			type: AlertType.Success,
			message: `${database.name} schema loaded`
		});
};

/**
 * Databases with a read in flight, so the tree, the tabs and the search menu
 * asking at once ask once. Cleared when the read settles, because a database
 * still empty then is meant to be asked again on the next click.
 */
const inFlight = new Set<string>();

/**
 * Loads the schema of a database that is showing nothing, so every click on
 * such a row is another attempt to open it. Empty is the one state where the
 * cached answer is the one that produced it.
 */
export const loadSchemaIfEmpty = async (database: graph.DBInstanceNode) => {
	if (database.children?.length || inFlight.has(database.id)) return;

	inFlight.add(database.id);
	try {
		await loadSchema({ database, announce: false });
	} finally {
		inFlight.delete(database.id);
	}
};
