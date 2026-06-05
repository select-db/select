import { writable, get } from 'svelte/store';

const ZOOM_KEY = 'app:zoom';
const STEP = 0.1;
const MIN = 0.5;
const MAX = 2.5;

const saved = parseFloat(localStorage.getItem(ZOOM_KEY) ?? '1') || 1;
export const zoomStore = writable(Math.min(MAX, Math.max(MIN, saved)));

function setZoom(factor: number): void {
	const clamped = Math.round(Math.min(MAX, Math.max(MIN, factor)) * 10) / 10;
	zoomStore.set(clamped);
	localStorage.setItem(ZOOM_KEY, String(clamped));
}

export const zoomIn = (): void => setZoom(get(zoomStore) + STEP);
export const zoomOut = (): void => setZoom(get(zoomStore) - STEP);
export const resetZoom = (): void => setZoom(1);
