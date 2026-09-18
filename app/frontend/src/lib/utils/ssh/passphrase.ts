import type { Component } from 'svelte';

import { modalStore } from '$lib/system/Modal/ModalStore';
import SSHPassphraseModal from '$lib/components/views/Database/SSHPassphraseModal.svelte';
import { SetSSHKeyPassphrase } from '$lib/bindings/selectDb/internal/db_client/dbclient';
import { GetDBInstanceNodeByID } from '$lib/wails/graph';
import { tryCatch } from '$lib/utils/tryCatch';

/** Whether a connection error indicates the SSH key file needs a passphrase. */
export const isEncryptedKeyError = (msg: string | null | undefined): boolean =>
	!!msg && /encrypted|passphrase/i.test(msg);

const promptPassphrase = (host = ''): Promise<string | null> =>
	new Promise((resolve) => {
		modalStore.set({
			content: (() => SSHPassphraseModal) as () => Component,
			props: {
				host,
				onSubmit: (value: string) => {
					modalStore.set(null);
					resolve(value);
				},
				onCancel: () => {
					modalStore.set(null);
					resolve(null);
				}
			},
			width: 360
		});
	});

/**
 * If `error` indicates an encrypted SSH key, prompt for the passphrase, store it
 * in memory keyed by the key file path (so later connects with the same key
 * reuse it), and return true, the caller should retry once. Returns false when
 * the error is unrelated, there is no key path, or the user cancels.
 */
export const ensureSSHPassphrase = async (
	keyPath: string | undefined,
	error: string | null | undefined,
	host = ''
): Promise<boolean> => {
	if (!keyPath || !isEncryptedKeyError(error)) return false;

	const pass = await promptPassphrase(host);
	if (pass === null) return false; // cancelled

	await tryCatch(SetSSHKeyPassphrase, keyPath, pass);

	return true;
};

/**
 * Same as ensureSSHPassphrase but resolves the key file path from a datasource
 * id (for callers that only know the instance, e.g. the query runner).
 */
export const ensureSSHPassphraseForInstance = async (
	dbInstanceId: string,
	error: string | null | undefined
): Promise<boolean> => {
	if (!isEncryptedKeyError(error)) return false;

	const [node] = await tryCatch(GetDBInstanceNodeByID, dbInstanceId);

	return ensureSSHPassphrase(node?.ssh?.key_path, error, node?.ssh?.host ?? '');
};
