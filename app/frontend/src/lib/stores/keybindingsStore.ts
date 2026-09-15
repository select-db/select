import { writable, get } from 'svelte/store';
import { EventsOn } from '$lib/wails/events';
import { tryCatch } from '$lib/utils/tryCatch';
import { notify } from '$lib/system/Notifications/notificationsStore';
import { AlertType } from '$lib/system/Alert/types';
import { setOS } from '$lib/utils/platform';
import type { KeybindingsContext } from './keybindingsContextStore';
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
type KeybindingProblem = keymapModels.Problem;
export type EditorSnippet = graphModels.EditorSnippet;
type ConfigData = graphModels.ConfigResponse;

export const keybindingsStore = writable<Keybinding[]>([]);
export const editorSnippetsStore = writable<EditorSnippet[]>([]);
export const configVersionStore = writable<number>(0);

function applyConfig(config: ConfigData | null | undefined): void {
	if (!config) return;
	if (config.keybindings) keybindingsStore.set(config.keybindings);
	if (config.editor_snippets !== undefined) editorSnippetsStore.set(config.editor_snippets);
	// The platform is a fact about the machine, so it comes from the backend
	// rather than from sniffing the user agent. Everything that needs it reads
	// it from there, `when` predicates included.
	if (config.os) setOS(config.os);
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
 * The punctuation a binding may name, as internal/keymap's `punctuation` lists
 * it: the symbols a US layout prints without shift. A keystroke that prints one
 * of these, or a letter or a digit, names a key a binding can be written for.
 */
const bindableCharacters = "-=[]\\;',./`";

function isBindableCharacter(key: string): boolean {
	return /^[a-z0-9]$/.test(key) || bindableCharacters.includes(key);
}

/**
 * The key a keystroke prints, when that is something a binding can name.
 *
 * This is what somebody reads off their own keyboard: a French layout prints
 * "-" on the key a US layout prints "=" on, and a binding written for "-" is
 * the one they expect that key to run. Empty when the character is not one a
 * binding can be written for -- "&" on AZERTY, "é", a dead key -- and the
 * position below answers for those.
 */
function printedKey(e: KeyboardEvent): string {
	const key = (e.key ?? '').toLowerCase();
	if (modifierKeys.has(e.key ?? '')) return '';
	if (key.length !== 1) return '';
	return isBindableCharacter(key) ? key : '';
}

/**
 * The key a keystroke sits on.
 *
 * Named by where it is, so a binding written for a key in one place is answered
 * by the key in the same place on a layout that prints something else there.
 * This is what a keystroke falls back to: the digit row needs shift on AZERTY,
 * holding alt prints another character again on macOS, and neither of those
 * prints a name a binding could be written for.
 */
function positionalKey(e: KeyboardEvent): string {
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

/** Modifiers in a fixed order, then the key: internal/keymap's canonical form. */
function chord(e: KeyboardEvent, key: string): string {
	const parts: string[] = [];
	if (e.ctrlKey) parts.push('ctrl');
	if (e.altKey) parts.push('alt');
	if (e.shiftKey) parts.push('shift');
	if (e.metaKey) parts.push('cmd');
	parts.push(key);

	return parts.join('+');
}

/**
 * The chords a keystroke answers to, in the order they are tried: what the key
 * prints first, where the key sits second.
 *
 * The two are the same string on a US layout and differ on every other, which
 * is the whole point: on AZERTY the key printed "-" answers "cmd+-" rather than
 * running whatever is bound to the key in that position, and a key that prints
 * nothing a binding could name still answers for its position.
 */
export function chordsFromEvent(e: KeyboardEvent): string[] {
	const printed = printedKey(e);
	const positional = positionalKey(e);

	const chords: string[] = [];
	if (printed) chords.push(chord(e, printed));
	if (positional && positional !== printed) chords.push(chord(e, positional));

	return chords;
}

function evaluateWhen(when: string | undefined, context: KeybindingsContext): boolean {
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

/**
 * The binding a keystroke runs.
 *
 * Every binding is tried against what the key prints before any of them is
 * tried against where the key sits, so a layout that prints "-" somewhere else
 * runs the binding written for "-" from that key, and a key printing nothing a
 * binding could name still runs what its position is bound to.
 */
export function keybindingForEvent(
	e: KeyboardEvent,
	context: KeybindingsContext
): Keybinding | null {
	for (const chord of chordsFromEvent(e)) {
		const keybinding = findMatchingKeybinding(chord, context);
		if (keybinding) return keybinding;
	}
	return null;
}
