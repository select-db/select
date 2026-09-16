import * as graph from '$lib/wails/graph';
import { QuerySchema } from '$lib/bindings/selectDb/internal/db_client/dbclient';

import { AlertType } from '$lib/system/Alert/types';
import { notify, notifyError } from '$lib/system/Notifications/notificationsStore';
import { pushToLoadingStore, removeFromLoadingStore } from './loadingStore';
import { tryCatch } from '../tryCatch';

/**
 * Loads a database's schema, reporting failure the way the rest of the app
 * reports failure.
 *
 * There used to be a `silent` option, taken by four of the nine call sites —
 * including both tree-click paths and the explicit "Reload schema" action — and
 * it suppressed the error, not the noise. A database that cannot be read then
 * expanded to nothing with the reason only in the log.
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

	// Always past the cache: every caller here is either a person asking for the
	// schema again or a database showing nothing, and the cached answer is what
	// they are asking to go behind.
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
 * asking at once ask once. Cleared when the read settles: a database that is
 * still empty afterwards is meant to be asked again on the next click.
 */
const reading = new Set<string>();

/**
 * Loads the schema of a database that is showing nothing.
 *
 * Empty is the one state where cached metadata is worth nothing: the entry the
 * cache would serve is the one that produced the empty row, so a person
 * clicking it again would be answered from the cache and see it stay empty.
 * The read goes to the database instead, which makes every click on such a row
 * another attempt to open it.
 */
export const loadSchemaIfEmpty = async (database: graph.DBInstanceNode) => {
	if (database.children?.length || reading.has(database.id)) return;

	reading.add(database.id);
	try {
		await loadSchema({ database, announce: false });
	} finally {
		reading.delete(database.id);
	}
};
