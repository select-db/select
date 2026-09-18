import { writable, get } from 'svelte/store';
import { osStore } from '$lib/utils/platform';

export type KeybindingsContext = {
	/**
	 * The platform, mirrored from `osStore`. A binding asks for it when a chord
	 * is conventional on one platform and taken on another -- `ctrl+-` is Go
	 * Back on macOS and zoom out everywhere else.
	 */
	os: string;
	inputFocus: boolean;
	editorFocus: boolean;
	menuFocus: boolean;
	modalOpen: boolean;
	leftPanelVisible: boolean;
	rightPanelVisible: boolean;
	activeView: string | null;
};

const defaultContext: KeybindingsContext = {
	os: '',
	inputFocus: false,
	editorFocus: false,
	menuFocus: false,
	modalOpen: false,
	leftPanelVisible: true,
	rightPanelVisible: false,
	activeView: null
};

export const keybindingsContext = writable<KeybindingsContext>({ ...defaultContext });

export function setContext<K extends keyof KeybindingsContext>(
	key: K,
	value: KeybindingsContext[K]
): void {
	keybindingsContext.update((ctx) => ({ ...ctx, [key]: value }));
}

export function getContext(): KeybindingsContext {
	return get(keybindingsContext);
}

export function resetContext(): void {
	keybindingsContext.set({ ...defaultContext });
}

// The platform is not something the app does, so nothing sets it here: it is
// mirrored from the one place that holds it.
osStore.subscribe((os) => setContext('os', os));
