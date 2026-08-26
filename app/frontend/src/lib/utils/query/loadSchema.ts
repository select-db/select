import * as graph from '$lib/wails/graph';
import { QuerySchema } from '$lib/bindings/selectDb/internal/db_client/dbclient';

import { AlertType } from '$lib/system/Alert/types';
import { notify, notifyError } from '$lib/system/Notifications/notificationsStore';
import {
	clearDatabaseError,
	setDatabaseError
} from '$lib/components/shared/DatabaseIndicator/databaseIndicatorStore';

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

	// The failure is recorded whether or not this call wanted an alert: a toast
	// is gone in seconds, and the thing the user is looking at is a database
	// that expanded to nothing. The tree reads this to say why.
	if (err) {
		setDatabaseError(database.id, err.message);
		if (!silent) notifyError(err.message);
		return;
	}

	clearDatabaseError(database.id);

	if (noCache)
		notify({
			type: AlertType.Success,
			message: `${database.name} schema loaded`
		});
};
