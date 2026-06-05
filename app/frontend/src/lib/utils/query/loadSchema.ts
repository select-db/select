import { graph } from '$lib/wailsjs/go/models';
import { QuerySchema } from '$lib/wailsjs/go/db_client/DbClient';

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

	// eslint-disable-next-line @typescript-eslint/no-unused-vars
	const [_, err] = await tryCatch(QuerySchema, {
		DatabaseInstanceID: database.id,
		NoCache: noCache
	});

	if (err && !silent) notifyError(err.message);

	removeFromLoadingStore(database.id);

	if (noCache)
		notify({
			type: AlertType.Success,
			message: `${database.name} schema loaded`
		});
};
