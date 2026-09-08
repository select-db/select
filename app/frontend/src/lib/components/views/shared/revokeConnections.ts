import type { Component } from 'svelte';
import { get } from 'svelte/store';

import { DeleteDatasource } from '$lib/bindings/selectDb/internal/datasource/datasource';
import type * as graph from '$lib/wails/graph';
import RevokeConnectionModal from '$lib/components/views/Database/RevokeConnectionModal.svelte';
import { modalStore } from '$lib/system/Modal/ModalStore';
import { notifyError } from '$lib/system/Notifications/notificationsStore';
import { myPermissions } from '$lib/stores/myPermissionsStore';
import { tryCatch } from '$lib/utils/tryCatch';

/**
 * Revokes shared connections: the permission check, the confirmation, and the
 * calls that actually drop the credentials.
 *
 * The rule belongs to the revoke, not to whichever gesture reaches it. Deleting
 * a database's folder is one way there, turning "Proxified" off is another, and
 * the connections screen a third; a credential dropped by any of them stops
 * every person in the workspace from querying, so all three ask the same
 * question first.
 *
 * Resolves true only when every named credential is gone. A false answer means
 * nothing was revoked and the caller should leave its own state alone --
 * refused, cancelled, or the server would not do it.
 *
 * `verb` names the gesture in the confirmation, because "Delete" and "Revoke"
 * are the same act here but not the same sentence.
 */
export const revokeConnections = async (
	connections: graph.DatabaseRef[],
	verb: 'Delete' | 'Revoke' = 'Revoke'
): Promise<boolean> => {
	if (connections.length === 0) return true;

	// Credentials are administrated, not owned by whoever has the workspace
	// open. The server refuses this too; refusing here is so the answer arrives
	// before anything else has happened.
	const { canManageDb } = get(myPermissions);
	const refused = connections.filter((db) => !canManageDb(db.id));
	if (refused.length > 0) {
		const one = refused.length === 1;
		notifyError(
			`${refused.map((db) => db.name).join(', ')} ${one ? 'is a shared connection' : 'are shared connections'}. Ask an admin: revoking the stored credentials is not yours to do.`
		);
		return false;
	}

	if (
		!(await confirm(
			connections.map((db) => db.name),
			verb
		))
	)
		return false;

	for (const db of connections) {
		const [, err] = await tryCatch(DeleteDatasource, db.id);
		if (err) {
			// Stopping at the first failure is deliberate: the caller has not
			// changed anything yet, so this leaves a state the person can retry
			// from rather than a half-revoked one they cannot see.
			notifyError(
				`The stored credentials for ${db.name} could not be revoked. Nothing else was changed.`
			);
			return false;
		}
	}

	return true;
};

const confirm = (names: string[], verb: 'Delete' | 'Revoke'): Promise<boolean> =>
	new Promise((resolve) => {
		const close = (answer: boolean) => {
			modalStore.set(null);
			resolve(answer);
		};
		modalStore.set({
			content: (() => RevokeConnectionModal) as () => Component,
			width: 520,
			props: {
				names,
				verb,
				onCancel: () => close(false),
				onConfirm: () => close(true)
			}
		});
	});
