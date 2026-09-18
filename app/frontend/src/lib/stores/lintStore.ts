import { writable } from 'svelte/store';
import { EventsOn } from '$lib/wails/events';

export const lintVersionStore = writable<number>(0);

EventsOn('lintUpdated', () => {
	lintVersionStore.update((v) => v + 1);
});
