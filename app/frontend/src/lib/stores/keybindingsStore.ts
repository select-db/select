import { writable, get } from 'svelte/store';
import { EventsOn } from '$lib/wailsjs/runtime/runtime';
import { tryCatch } from '$lib/utils/tryCatch';
import { isMac } from '$lib/utils/platform';
import type { KeybindingsContext } from './keybindingsContextStore';
import { GetConfig } from '$lib/wailsjs/go/system/System';

export type Keybinding = {
	key: string;
	command: string;
	when?: string;
};

export type EditorSnippet = {
	prefix: string;
	body: string;
	description?: string;
};

export type ConfigData = {
	keybindings: Keybinding[];
	editor_snippets?: EditorSnippet[];
};

export const keybindingsStore = writable<Keybinding[]>([]);
export const editorSnippetsStore = writable<EditorSnippet[]>([]);
export const configVersionStore = writable<number>(0);

function applyConfig(config: ConfigData | null | undefined): void {
	if (!config) return;
	if (config.keybindings) keybindingsStore.set(config.keybindings);
	if (config.editor_snippets !== undefined) editorSnippetsStore.set(config.editor_snippets);
}

export async function initKeybindings(): Promise<void> {
	const [config] = await tryCatch(GetConfig);
	applyConfig(config ?? undefined);
}

EventsOn('configUpdated', (data: ConfigData) => {
	applyConfig(data);
	configVersionStore.update((v) => v + 1);
});

export function normalizeKey(e: KeyboardEvent): string {
	const parts: string[] = [];

	if (e.metaKey && isMac) parts.push('cmd');
	if (e.ctrlKey && !isMac) parts.push('cmd');
	if (e.ctrlKey && isMac) parts.push('ctrl');
	if (e.shiftKey) parts.push('shift');
	if (e.altKey) parts.push('alt');

	const key = getKey(e);
	if (!['control', 'meta', 'alt', 'shift'].includes(key)) {
		parts.push(key);
	}

	return parts.join('+');
}

function getKey(e: KeyboardEvent): string {
	const code = e.code ?? '';
	const key = e.key ?? '';

	if (code.startsWith('Arrow')) return code.toLowerCase();
	if (/^F\d+$/.test(code)) return code.toLowerCase();

	const codeMap: Record<string, string> = {
		Space: 'space',
		Enter: 'enter',
		Escape: 'escape',
		Backspace: 'backspace',
		Delete: 'delete',
		Tab: 'tab',
		Home: 'home',
		End: 'end',
		PageUp: 'pageup',
		PageDown: 'pagedown',
		Insert: 'insert'
	};
	if (code in codeMap) return codeMap[code];

	// Use e.key for all printable ASCII so bindings respect the active keyboard layout (AZERTY, QWERTZ, etc.)
	if (key.length === 1) {
		const charCode = key.charCodeAt(0);
		if (charCode >= 0x20 && charCode <= 0x7e) {
			return key.toLowerCase();
		}
	}

	// Fallback for non-printable e.key (unicode from Option+key, dead keys, etc.)
	if (code.startsWith('Key')) return code.slice(3).toLowerCase();
	if (code.startsWith('Digit')) return code.slice(5);

	return key.toLowerCase();
}

export function evaluateWhen(when: string | undefined, context: KeybindingsContext): boolean {
	if (!when || when.trim() === '') return true;
	const [result, err] = tryCatch(() => evaluateTokens(tokenizeWhen(when), context));
	return err ? false : result;
}

type Token =
	| { type: 'identifier'; value: string }
	| { type: 'operator'; value: string }
	| { type: 'string'; value: string }
	| { type: 'lparen' }
	| { type: 'rparen' };

function tokenizeWhen(when: string): Token[] {
	const tokens: Token[] = [];
	let i = 0;

	while (i < when.length) {
		const ch = when[i];

		if (/\s/.test(ch)) {
			i++;
			continue;
		}

		if (ch === '(') {
			tokens.push({ type: 'lparen' });
			i++;
			continue;
		}

		if (ch === ')') {
			tokens.push({ type: 'rparen' });
			i++;
			continue;
		}

		if (ch === '!' && when[i + 1] !== '=') {
			tokens.push({ type: 'operator', value: '!' });
			i++;
			continue;
		}

		if (ch === '&' && when[i + 1] === '&') {
			tokens.push({ type: 'operator', value: '&&' });
			i += 2;
			continue;
		}

		if (ch === '|' && when[i + 1] === '|') {
			tokens.push({ type: 'operator', value: '||' });
			i += 2;
			continue;
		}

		if (ch === '=' && when[i + 1] === '=') {
			tokens.push({ type: 'operator', value: '==' });
			i += 2;
			continue;
		}

		if (ch === '!' && when[i + 1] === '=') {
			tokens.push({ type: 'operator', value: '!=' });
			i += 2;
			continue;
		}

		if (ch === "'" || ch === '"') {
			const quote = ch;
			let str = '';
			i++;
			while (i < when.length && when[i] !== quote) {
				str += when[i];
				i++;
			}
			i++;
			tokens.push({ type: 'string', value: str });
			continue;
		}

		if (/[a-zA-Z_]/.test(ch)) {
			let ident = '';
			while (i < when.length && /[a-zA-Z0-9_]/.test(when[i])) {
				ident += when[i];
				i++;
			}
			tokens.push({ type: 'identifier', value: ident });
			continue;
		}

		i++;
	}

	return tokens;
}

function evaluateTokens(tokens: Token[], context: KeybindingsContext): boolean {
	let pos = 0;

	const peek = (): Token | undefined => tokens[pos];
	const consume = (): Token | undefined => tokens[pos++];

	const parseOr = (): boolean => {
		let left = parseAnd();
		while (peek()?.type === 'operator' && (peek() as { value: string }).value === '||') {
			consume();
			left = left || parseAnd();
		}
		return left;
	};

	const parseAnd = (): boolean => {
		let left = parseUnary();
		while (peek()?.type === 'operator' && (peek() as { value: string }).value === '&&') {
			consume();
			left = left && parseUnary();
		}
		return left;
	};

	const parseUnary = (): boolean => {
		const token = peek();
		if (token?.type === 'operator' && token.value === '!') {
			consume();
			return !parseUnary();
		}
		return parsePrimary();
	};

	const parsePrimary = (): boolean => {
		const token = peek();

		if (token?.type === 'lparen') {
			consume();
			const result = parseOr();
			if (peek()?.type === 'rparen') consume();
			return result;
		}

		if (token?.type === 'identifier') {
			consume();
			const ident = token.value;
			const nextToken = peek();

			if (
				nextToken?.type === 'operator' &&
				(nextToken.value === '==' || nextToken.value === '!=')
			) {
				const op = (consume() as { value: string }).value;
				const valueToken = consume();
				const compareValue =
					valueToken?.type === 'string'
						? valueToken.value
						: valueToken?.type === 'identifier'
							? valueToken.value
							: '';

				const contextValue = context[ident as keyof KeybindingsContext];
				return op === '=='
					? String(contextValue) === compareValue
					: String(contextValue) !== compareValue;
			}

			return Boolean(context[ident as keyof KeybindingsContext]);
		}

		return false;
	};

	return parseOr();
}

export function findMatchingKeybinding(
	key: string,
	context: KeybindingsContext
): Keybinding | null {
	const keybindings = get(keybindingsStore);
	for (const kb of keybindings) {
		if (kb.key === key && evaluateWhen(kb.when, context)) {
			return kb;
		}
	}
	return null;
}
