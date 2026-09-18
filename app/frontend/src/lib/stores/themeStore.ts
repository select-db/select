import { writable, get } from 'svelte/store';
import { EventsOn } from '$lib/wails/events';
import { GetThemeVariables } from '$lib/bindings/selectDb/internal/system/system';
import { tryCatch } from '$lib/utils/tryCatch';

// Values are optional because the bindings model Go maps as possibly-missing
// keys; entries without a value are skipped when applying the theme.
export type ThemeVariables = {
	shared: Record<string, string | undefined>;
	light: Record<string, string | undefined>;
	dark: Record<string, string | undefined>;
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

	const apply = (vars: Record<string, string | undefined>) => {
		for (const [varName, value] of Object.entries(vars)) {
			if (value === undefined) continue;
			root.style.setProperty(varName, value);
			appliedVarNames.add(varName);
		}
	};

	apply(variables.shared);
	apply(mode === 'dark' ? variables.dark : variables.light);

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
