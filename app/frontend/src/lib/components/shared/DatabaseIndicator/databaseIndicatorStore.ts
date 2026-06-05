import { EventsOn } from '$lib/wailsjs/runtime/runtime';
import { get, writable } from 'svelte/store';

export const databaseAvailabilityStore = writable<Set<string>>(new Set());

EventsOn('databaseAvailability', (data: { databases: { id: string; error?: string }[] }) => {
	const set = get(databaseAvailabilityStore);
	for (const { id, error } of data.databases) {
		if (error) set.delete(id);
		else set.add(id);
	}
	databaseAvailabilityStore.set(set);
});
