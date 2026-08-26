import * as graph from '$lib/wails/graph';
import { QuerySchema } from '$lib/bindings/selectDb/internal/db_client/dbclient';

import { AlertType } from '$lib/system/Alert/types';
import { notify, notifyError } from '$lib/system/Notifications/notificationsStore';
import { pushToLoadingStore, removeFromLoadingStore } from './loadingStore';
import { tryCatch } from '../tryCatch';

export const loadSchema = async ({
	database,
	noCache = false,
	silent = false
}: {
	database: graph.DBInstanceNode;
	noCache?: boolean;
	/** When true, do not show an error alert */
	silent?: boolean;
}) => {
	pushToLoadingStore(database.id);

	const [, err] = await tryCatch(QuerySchema, {
		DatabaseInstanceID: database.id,
		NoCache: noCache
	});

	removeFromLoadingStore(database.id);

	// A failed load reports and stops: announcing "schema loaded" straight after
	// an error is what made this read as a silent failure.
	if (err) {
		if (!silent) notifyError(err.message);
		return;
	}

	if (noCache)
		notify({
			type: AlertType.Success,
			message: `${database.name} schema loaded`
		});
};
