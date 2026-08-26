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
	noCache = false
}: {
	database: graph.DBInstanceNode;
	noCache?: boolean;
}) => {
	pushToLoadingStore(database.id);

	const [, err] = await tryCatch(QuerySchema, {
		DatabaseInstanceID: database.id,
		NoCache: noCache
	});

	removeFromLoadingStore(database.id);

	// Reports and stops: announcing "schema loaded" straight after an error is
	// what made this read as a silent failure.
	if (err) {
		notifyError(err.message);
		return;
	}

	if (noCache)
		notify({
			type: AlertType.Success,
			message: `${database.name} schema loaded`
		});
};
