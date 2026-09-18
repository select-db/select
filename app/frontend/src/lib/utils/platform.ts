import { get, writable } from 'svelte/store';

/**
 * What platform the app is running on.
 *
 * From the backend, which is the one that knows: it is a binary built for this
 * operating system, where the frontend can only read `navigator`, whose
 * `platform` is deprecated and whose user agent is a guess. The value arrives
 * with the personal config, and is the same string a keybinding's `when` reads
 * as `os`.
 */
export type OS = 'macos' | 'linux' | 'windows';

export const osStore = writable<OS | ''>('');

export function setOS(os: string): void {
	if (os === 'macos' || os === 'linux' || os === 'windows') osStore.set(os);
}

export function isMac(): boolean {
	return get(osStore) === 'macos';
}
