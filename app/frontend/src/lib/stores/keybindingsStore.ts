import { writable } from 'svelte/store';
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
type Keybinding = keymapModels.Binding;
type KeybindingProblem = keymapModels.Problem;
type EditorSnippet = graphModels.EditorSnippet;
type ConfigData = graphModels.ConfigResponse;

/** Read on every keystroke and by nothing else, so a variable rather than a store. */
let keybindings: Keybinding[] = [];
export const editorSnippetsStore = writable<EditorSnippet[]>([]);
export const configVersionStore = writable<number>(0);

function applyConfig(config: ConfigData | null | undefined): void {
	if (!config) return;
	if (config.keybindings) keybindings = config.keybindings;
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
			message: describeProblems(errors, 'could not be read'),
			duration: 8000,
			copyable: true
		});
	}

	const migrated = problems.filter((p) => p.level === 'migrated');
	if (migrated.length) {
		notify({
			type: AlertType.Default,
			message: describeProblems(migrated, 'in your config need updating'),
			duration: 8000
		});
	}
}

/** The backend's message for one problem, the keys for several. */
function describeProblems(problems: KeybindingProblem[], summary: string): string {
	if (problems.length === 1) return `Keybinding ${problems[0].key}: ${problems[0].message}`;
	return `${problems.length} keybindings ${summary}: ${problems.map((p) => p.key).join(', ')}`;
}

export async function initKeybindings(): Promise<void> {
	const [config] = await tryCatch(GetConfig);
	applyConfig(config);
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
 * The punctuation a binding may name: the symbols a US layout prints unshifted,
 * which are the one-character names in keyByCode. Read from there rather than
 * listed again, so the two cannot disagree.
 */
const bindableCharacters = new Set(Object.values(keyByCode).filter((name) => name.length === 1));

/**
 * The key a keystroke prints, when a binding can name that.
 *
 * What somebody reads off their own keyboard: a French layout prints "-" where
 * a US one prints "=", and "cmd+-" is what they expect that key to run. Empty
 * for a character no binding can name, which the position below answers for.
 */
function printedKey(e: KeyboardEvent): string {
	const key = (e.key ?? '').toLowerCase();
	return /^[a-z0-9]$/.test(key) || bindableCharacters.has(key) ? key : '';
}

/**
 * The key a keystroke sits on, named by where it is rather than what it prints.
 *
 * What a keystroke falls back to: the AZERTY digit row needs shift and alt
 * prints something else again on macOS, and neither prints a bindable name.
 */
function positionalKey(e: KeyboardEvent): string {
	const code = e.code ?? '';
	const key = e.key ?? '';

	if (modifierKeys.has(key)) return '';

	if (/^Key[A-Z]$/.test(code)) {
		return /^[a-z]$/i.test(key) ? key.toLowerCase() : code.slice(3).toLowerCase();
	}
	const digit = /^(?:Digit|Numpad)([0-9])$/.exec(code);
	if (digit) return digit[1];
	if (/^F([1-9]|1[0-9]|2[0-4])$/.test(code)) return code.toLowerCase();
	if (code in keyByCode) return keyByCode[code];

	return key.toLowerCase();
}

/** Modifiers in a fixed order, then the key, as internal/keymap prints a chord. */
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
 * The chords a keystroke answers to: what the key prints first, where it sits
 * second. The two are one string on a US layout and differ on every other.
 */
function chordsFromEvent(e: KeyboardEvent): string[] {
	const printed = printedKey(e);
	const positional = positionalKey(e);

	const chords: string[] = [];
	if (printed) chords.push(chord(e, printed));
	if (positional && positional !== printed) chords.push(chord(e, positional));

	return chords;
}

function evaluateWhen(when: string | undefined, context: KeybindingsContext): boolean {
	if (!when?.trim()) return true;
	const [result, err] = tryCatch(() => evaluateTokens(tokenizeWhen(when), context));
	return err ? false : result;
}

type Token = { type: 'op' | 'identifier' | 'string'; value: string };

/**
 * Whitespace, an operator, a quoted string whose closing quote may be missing,
 * an identifier, or any other character, which is skipped.
 */
const WHEN_TOKEN = /\s+|(!=|==|&&|\|\||[!()])|'([^']*)'?|"([^"]*)"?|([A-Za-z_]\w*)|[^]/g;

function tokenizeWhen(when: string): Token[] {
	const tokens: Token[] = [];
	for (const [, op, singleQuoted, doubleQuoted, identifier] of when.matchAll(WHEN_TOKEN)) {
		const quoted = singleQuoted ?? doubleQuoted;
		if (op) tokens.push({ type: 'op', value: op });
		else if (quoted !== undefined) tokens.push({ type: 'string', value: quoted });
		else if (identifier) tokens.push({ type: 'identifier', value: identifier });
	}
	return tokens;
}

function evaluateTokens(tokens: Token[], context: KeybindingsContext): boolean {
	let pos = 0;

	const eat = (op: string): boolean => {
		const token = tokens[pos];
		if (token?.type !== 'op' || token.value !== op) return false;
		pos++;
		return true;
	};

	const parseOr = (): boolean => {
		let left = parseAnd();
		while (eat('||')) left = left || parseAnd();
		return left;
	};

	const parseAnd = (): boolean => {
		let left = parseUnary();
		while (eat('&&')) left = left && parseUnary();
		return left;
	};

	const parseUnary = (): boolean => (eat('!') ? !parseUnary() : parsePrimary());

	const parsePrimary = (): boolean => {
		if (eat('(')) {
			const result = parseOr();
			eat(')');
			return result;
		}

		const token = tokens[pos];
		if (token?.type !== 'identifier') return false;
		pos++;

		const contextValue = context[token.value as keyof KeybindingsContext];
		const equals = eat('==');
		if (!equals && !eat('!=')) return Boolean(contextValue);

		const compared = tokens[pos++];
		const compareValue = compared && compared.type !== 'op' ? compared.value : '';
		return (String(contextValue) === compareValue) === equals;
	};

	return parseOr();
}

/**
 * The binding a keystroke runs, or null when nothing is bound to it. Every
 * binding is tried against what the key prints before any is tried against
 * where the key sits.
 *
 * The list arrives in increasing precedence -- the defaults laid out category by
 * category, then whatever the person wrote themselves -- so the last binding
 * that fits is the one that wins. A binding with no command is an unbinding, and
 * is returned as one: it stops the search rather than falling through to the
 * default it was written to take away.
 */
export function keybindingForEvent(
	e: KeyboardEvent,
	context: KeybindingsContext
): Keybinding | null {
	for (const chord of chordsFromEvent(e)) {
		const keybinding = keybindings.findLast(
			(kb) => kb.key === chord && evaluateWhen(kb.when, context)
		);
		if (keybinding) return keybinding;
	}
	return null;
}
