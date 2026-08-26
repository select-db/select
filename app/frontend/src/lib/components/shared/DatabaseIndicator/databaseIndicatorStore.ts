import { EventsOn } from '$lib/wails/events';
import { get, writable } from 'svelte/store';

export const databaseAvailabilityStore = writable<Set<string>>(new Set());

/**
 * Why a database is not usable, keyed by instance id.
 *
 * Connecting and reading a schema are separate things, and the second can fail
 * on a database that answers a ping perfectly well — a role without rights on a
 * table, or, as reported, a server whose remaining connection slots are
 * reserved for superusers. The backend logs those and returns them; nothing was
 * showing them, so the tree simply stayed empty and the connection dot stayed
 * green, which reads as "the app is broken" rather than "the server said no".
 */
export const databaseErrorsStore = writable<Map<string, string>>(new Map());

export function setDatabaseError(id: string, message: string) {
	databaseErrorsStore.update((errors) => new Map(errors).set(id, message));
}

export function clearDatabaseError(id: string) {
	databaseErrorsStore.update((errors) => {
		if (!errors.has(id)) return errors;
		const next = new Map(errors);
		next.delete(id);
		return next;
	});
}

EventsOn('databaseAvailability', (data: { databases: { id: string; error?: string }[] }) => {
	const set = get(databaseAvailabilityStore);
	for (const { id, error } of data.databases) {
		if (error) {
			set.delete(id);
			setDatabaseError(id, error);
		} else {
			set.add(id);
			clearDatabaseError(id);
		}
	}
	databaseAvailabilityStore.set(set);
});
