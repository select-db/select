import { EventsOn } from '$lib/wails/events';
import { get, writable } from 'svelte/store';

/**
 * What each database was last found to be, by whatever last reached it: a ping,
 * a schema load, a query that ran. The Go side reports every such outcome (see
 * internal/db_client/availability.go), so this is the app's whole picture of
 * which databases answer.
 *
 * A database missing from the map has not been reached yet, which is not the
 * same as one that failed. The indicator shows the two differently: a database
 * nobody has tried is not a database that is down.
 */
export const databaseAvailabilityStore = writable<Map<string, boolean>>(new Map());

EventsOn('databaseAvailability', (data: { databases: { id: string; error?: string }[] }) => {
	const available = new Map(get(databaseAvailabilityStore));
	for (const { id, error } of data.databases) {
		available.set(id, !error);
	}
	databaseAvailabilityStore.set(available);
});
