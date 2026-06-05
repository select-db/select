import { writable } from 'svelte/store';

export type UpdateStatus = 'available' | 'downloading' | 'installing' | 'error';

export type UpdateState = {
	status: UpdateStatus;
	progress: number;
	version: string;
	message: string;
} | null;

export const updateStore = writable<UpdateState>(null);
