import { writable, get } from 'svelte/store';
import { EventsOn } from '$lib/wails/events';
import { tryCatch } from '$lib/utils/tryCatch';
import { notify } from '$lib/system/Notifications/notificationsStore';
import { AlertType } from '$lib/system/Alert/types';
import { setContext, type KeybindingsContext } from './keybindingsContextStore';
import { GetConfig } from '$lib/bindings/selectDb/internal/system/system';
import type * as graphModels from '$lib/bindings/selectDb/internal/graph/models';
import type * as keymapModels from '$lib/bindings/selectDb/internal/keymap/models';

/**
 * The shapes come from the backend rather than being written again here: a
 * binding arrives with its chord already parsed and resolved for this platform
 * -- `secondary` turned into the modifier this machine uses -- and nothing in
 * the frontend decides what a modifier means. See internal/keymap.
 */
export type Keybinding = keymapModels.Binding;
export type KeybindingProblem = keymapModels.Problem;
export type EditorSnippet = graphModels.EditorSnippet;
export type ConfigData = graphModels.ConfigResponse;

export const keybindingsStore = writable<Keybinding[]>([]);
export const editorSnippetsStore = writable<EditorSnippet[]>([]);
export const configVersionStore = writable<number>(0);

function applyConfig(config: ConfigData | null | undefined): void {
	if (!config) return;
	if (config.keybindings) keybindingsStore.set(config.keybindings);
	if (config.editor_snippets !== undefined) editorSnippetsStore.set(config.editor_snippets);
	// `os` is a fact about the machine, so it comes from the backend rather than
	// from sniffing the user agent, and bindings can ask for it: a chord that is
	// conventional on one platform and taken on another says so in its `when`.
	if (config.os) setContext('os', config.os);
	reportProblems(config.problems ?? []);
}

/**
 * Says what could not be read.
 *
 * A keybinding that fails to parse is invisible otherwise: the key simply never
 * does anything, which reads as the app ignoring it rather than as a line in a
 * file to fix.
 */
function reportProblems(problems: KeybindingProblem[]): void {
	const errors = problems.filter((p) => p.level === 'error');
	if (errors.length) {
		notify({
			type: AlertType.Error,
			message:
				errors.length === 1
					? `Keybinding ${errors[0].key}: ${errors[0].message}`
					: `${errors.length} keybindings could not be read: ${errors.map((p) => p.key).join(', ')}`,
			duration: 8000,
			copyable: true
		});
	}

	const migrated = problems.filter((p) => p.level === 'migrated');
	if (migrated.length) {
		notify({
			type: AlertType.Default,
			message: `${migrated.length} keybinding${migrated.length === 1 ? '' : 's'} in your config still say "cmd"; read as "secondary", the shortcut key on this platform`,
			duration: 8000
		});
	}
}

export async function initKeybindings(): Promise<void> {
	const [config] = await tryCatch(GetConfig);
	applyConfig(config ?? undefined);
}

EventsOn('configUpdated', (data: ConfigData) => {
	applyConfig(data);
	configVersionStore.update((v) => v + 1);
});

/** Keys named by where they are, for everything that is not a letter. */
const keyByCode: Record<string, string> = {
	Minus: '-',
	Equal: '=',
	BracketLeft: '[',
	BracketRight: ']',
	Backslash: '\\',
	Semicolon: ';',
	Quote: "'",
	Comma: ',',
	Period: '.',
	Slash: '/',
	Backquote: '`',
	Space: 'space',
	Enter: 'enter',
	NumpadEnter: 'enter',
	Escape: 'escape',
	Backspace: 'backspace',
	Delete: 'delete',
	Tab: 'tab',
	Home: 'home',
	End: 'end',
	PageUp: 'pageup',
	PageDown: 'pagedown',
	Insert: 'insert',
	ArrowUp: 'up',
	ArrowDown: 'down',
	ArrowLeft: 'left',
	ArrowRight: 'right'
};

const modifierKeys = new Set(['Control', 'Shift', 'Alt', 'Meta', 'CapsLock']);

/**
 * The key a keystroke names.
 *
 * A letter is what the layout prints -- somebody on AZERTY pressing the key
 * marked A means A. Everything else is named by where it sits: the digit row
 * needs shift on AZERTY and the punctuation moves on QWERTZ, so a binding
 * written once for "-" is answered by the key in the same place on any of them.
 * Holding alt prints a different character again on macOS, which is the other
 * reason a letter falls back to its position.
 */
function keyFromEvent(e: KeyboardEvent): string {
	const code = e.code ?? '';
	const key = e.key ?? '';

	if (modifierKeys.has(key)) return '';

	if (/^Key[A-Z]$/.test(code)) {
		return /^[a-z]$/i.test(key) ? key.toLowerCase() : code.slice(3).toLowerCase();
	}
	if (/^Digit[0-9]$/.test(code)) return code.slice(5);
	if (/^Numpad[0-9]$/.test(code)) return code.slice(6);
	if (/^F([1-9]|1[0-9]|2[0-4])$/.test(code)) return code.toLowerCase();
	if (code in keyByCode) return keyByCode[code];

	return key.toLowerCase();
}

/**
 * The keystroke, written the way a binding is: modifiers in a fixed order, then
 * the key. Matches internal/keymap's canonical form, and is compared to it as a
 * string -- the two orders are the same list.
 */
export function chordFromEvent(e: KeyboardEvent): string {
	const key = keyFromEvent(e);
	if (!key) return '';

	const parts: string[] = [];
	if (e.ctrlKey) parts.push('ctrl');
	if (e.altKey) parts.push('alt');
	if (e.shiftKey) parts.push('shift');
	if (e.metaKey) parts.push('cmd');
	parts.push(key);

	return parts.join('+');
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

/**
 * The binding a keystroke runs, or null when nothing is bound to it.
 *
 * Read from the end: the list arrives in increasing precedence -- the defaults
 * laid out category by category, then whatever the person wrote themselves --
 * so the last binding that fits is the one that wins. A binding with no command
 * is an unbinding, and is returned as one: it stops the search rather than
 * falling through to the default it was written to take away.
 */
export function findMatchingKeybinding(
	chord: string,
	context: KeybindingsContext
): Keybinding | null {
	const keybindings = get(keybindingsStore);
	for (let i = keybindings.length - 1; i >= 0; i--) {
		const kb = keybindings[i];
		if (kb.key === chord && evaluateWhen(kb.when, context)) return kb;
	}
	return null;
}
