import { writable, get } from 'svelte/store';

export type KeybindingsContext = {
	inputFocus: boolean;
	editorFocus: boolean;
	menuFocus: boolean;
	modalOpen: boolean;
	leftPanelVisible: boolean;
	rightPanelVisible: boolean;
	activeView: string | null;
};

const defaultContext: KeybindingsContext = {
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
