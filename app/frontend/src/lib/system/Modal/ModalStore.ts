import type { Component } from 'svelte';
import { writable } from 'svelte/store';

type ModalState = {
	content?: () => Component;
	props?: Record<string, unknown>;
	// number is treated as px; a string is used verbatim as a CSS length
	// (e.g. 'min(80vw, 860px)') so callers can size against the window
	width?: number | string;
	height?: number | string;
};

export const modalStore = writable<ModalState | null>(null);
