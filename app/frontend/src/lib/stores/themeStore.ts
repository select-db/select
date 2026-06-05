import { writable, get } from 'svelte/store';
import { EventsOn } from '$lib/wailsjs/runtime/runtime';
import { GetThemeVariables } from '$lib/wailsjs/go/system/System';
import { tryCatch } from '$lib/utils/tryCatch';

export type ThemeVariables = {
	shared: Record<string, string>;
	light: Record<string, string>;
	dark: Record<string, string>;
};

export const themeVersionStore = writable<number>(0);

const themeVariablesStore = writable<ThemeVariables>({
	shared: {},
	light: {},
	dark: {}
});

const appliedVarNames: Set<string> = new Set();

function getCurrentThemeMode(): 'light' | 'dark' {
	return document.documentElement.getAttribute('data-theme') === 'dark' ? 'dark' : 'light';
}

function applyThemeVariables(variables: ThemeVariables): void {
	const root = document.documentElement;
	const mode = getCurrentThemeMode();

	for (const varName of appliedVarNames) {
		root.style.removeProperty(varName);
	}
	appliedVarNames.clear();

	for (const [varName, value] of Object.entries(variables.shared)) {
		root.style.setProperty(varName, value);
		appliedVarNames.add(varName);
	}

	const modeVars = mode === 'dark' ? variables.dark : variables.light;
	for (const [varName, value] of Object.entries(modeVars)) {
		root.style.setProperty(varName, value);
		appliedVarNames.add(varName);
	}

	themeVariablesStore.set(variables);
	themeVersionStore.update((v) => v + 1);
}

function reapplyForCurrentMode(): void {
	const variables = get(themeVariablesStore);
	if (
		Object.keys(variables.shared).length > 0 ||
		Object.keys(variables.light).length > 0 ||
		Object.keys(variables.dark).length > 0
	) {
		applyThemeVariables(variables);
	}
}

export async function initTheme(): Promise<void> {
	const [variables] = await tryCatch(GetThemeVariables);
	if (variables) {
		applyThemeVariables(variables);
	}

	const observer = new MutationObserver((mutations) => {
		for (const mutation of mutations) {
			if (mutation.attributeName !== 'data-theme') continue;
			reapplyForCurrentMode();
		}
	});

	observer.observe(document.documentElement, { attributes: true });
}

EventsOn('themeUpdated', (data: ThemeVariables) => {
	applyThemeVariables(data);
});

export { themeVariablesStore };
