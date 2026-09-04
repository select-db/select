import type * as monaco from 'monaco-editor';

/**
 * Where the cursor is, as far as a completion provider needs to care.
 *
 * Three providers answer for SQL and each is only right in some places: the SQL
 * provider knows the schema and answers everywhere, the variable provider only
 * inside a `$` reference, snippets only where a statement could start. Reading
 * that off one string in one place is what stops them contradicting each other.
 * A snippet offered after `o.` sorts above the columns of `o` and is never what
 * was being asked for.
 */
export type CompletionContext =
	/** Inside a `$VARIABLE` reference: `$`, `$WARE`. */
	| 'variable'
	/** After a qualifier dot, so only members of what precedes it: `o.`, `main.orders.`. */
	| 'member'
	/** Anywhere else: a bare word, whitespace, the start of a line. */
	| 'open';

const VARIABLE = /\$[A-Za-z0-9_]*$/;

// An identifier, or the closing quote of one, then a dot, then as much of the
// member name as has been typed. Whitespace around the dot is legal SQL, and a
// digit before it is a decimal literal rather than a qualifier, which is not a
// place for a snippet or a variable either.
const MEMBER = /[\w"`\]]\s*\.\s*\w*$/;

/** The line up to the cursor, which is all either pattern looks at. */
export function textBeforeCursor(
	model: monaco.editor.ITextModel,
	position: monaco.IPosition
): string {
	return model.getValueInRange({
		startLineNumber: position.lineNumber,
		startColumn: 1,
		endLineNumber: position.lineNumber,
		endColumn: position.column
	});
}

export function contextAt(text: string): CompletionContext {
	if (VARIABLE.test(text)) return 'variable';
	if (MEMBER.test(text)) return 'member';
	return 'open';
}
