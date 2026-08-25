/**
 * Window zoom, applied by the webview itself.
 *
 * The backend drives the webview's page zoom — the reflowing kind a browser
 * applies with Cmd/Ctrl +/- — so the whole UI re-lays out and no layout maths
 * in the app has to compensate for it.
 *
 * Zoom is stepped in levels rather than raw factors, the way editors do it:
 * each step is 20%, so a press feels the same size at any level. The level is
 * remembered because the webview starts at 100% on every launch.
 */
import { SetZoom } from '$lib/bindings/selectDb/internal/system/system';
import { notifyError } from '$lib/system/Notifications/notificationsStore';

const STEP = 1.2;
const MIN_LEVEL = -3; // ~0.58x
const MAX_LEVEL = 5; // ~2.49x

const LEVEL_KEY = 'app:zoomLevel';
/** Pre-level key, holding a raw factor. Read once, then migrated away. */
const LEGACY_FACTOR_KEY = 'app:zoom';

const factorFor = (level: number): number => Math.pow(STEP, level);
const levelFor = (factor: number): number => Math.round(Math.log(factor) / Math.log(STEP));
const clampLevel = (level: number): number => Math.min(MAX_LEVEL, Math.max(MIN_LEVEL, level));

function readSavedLevel(): number {
	const saved = parseInt(localStorage.getItem(LEVEL_KEY) ?? '', 10);
	if (Number.isFinite(saved)) return clampLevel(saved);

	const legacyFactor = parseFloat(localStorage.getItem(LEGACY_FACTOR_KEY) ?? '');
	localStorage.removeItem(LEGACY_FACTOR_KEY);
	if (Number.isFinite(legacyFactor) && legacyFactor > 0) return clampLevel(levelFor(legacyFactor));

	return 0;
}

let level = 0;

async function applyLevel(next: number): Promise<void> {
	const wanted = clampLevel(next);

	// The backend reports the factor it managed to apply: Windows cannot zoom
	// below 100%, so the level follows what the webview actually did.
	let applied: number;
	try {
		applied = await SetZoom(factorFor(wanted));
	} catch (err) {
		notifyError(`Could not zoom: ${err}`);
		return;
	}

	level = Number.isFinite(applied) && applied > 0 ? clampLevel(levelFor(applied)) : wanted;
	localStorage.setItem(LEVEL_KEY, String(level));
}

/** Restores the zoom level the user last chose. Called once, at startup. */
export async function initZoom(): Promise<void> {
	level = readSavedLevel();
	if (level === 0) return;

	await applyLevel(level);
}

export const zoomIn = (): Promise<void> => applyLevel(level + 1);
export const zoomOut = (): Promise<void> => applyLevel(level - 1);
export const resetZoom = (): Promise<void> => applyLevel(0);
