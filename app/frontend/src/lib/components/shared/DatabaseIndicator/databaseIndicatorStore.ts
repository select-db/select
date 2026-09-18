import { EventsOn } from '$lib/wails/events';
import { get, writable } from 'svelte/store';

/**
 * What each database was last found to be, by whatever last reached it. A
 * database missing from the map has not been reached yet, which is not the same
 * as one that failed, and the indicator shows the two differently.
 */
export const databaseAvailabilityStore = writable<Map<string, boolean>>(new Map());

EventsOn('databaseAvailability', (data: { databases: { id: string; error?: string }[] }) => {
	const available = new Map(get(databaseAvailabilityStore));
	for (const { id, error } of data.databases) {
		available.set(id, !error);
	}
	databaseAvailabilityStore.set(available);
});
