import { marked } from 'marked';
import markedKatex from 'marked-katex-extension';
import DOMPurify, { type Config } from 'dompurify';

let katexConfigured = false;

/**
 * Deny-by-default sanitiser settings for chat markdown.
 *
 * This is the only barrier between a model's reply and the DOM: `marked` has no
 * sanitiser of its own, and the reply is not trusted input — the chat tools feed
 * query results to the model, so a value stored in a table the user inspects can
 * steer what comes back. The result renders inside the Wails webview, where the
 * services in `main.go` are bound to JS, so script execution here reaches the
 * filesystem, the terminal and the keyring rather than just a page.
 *
 * Everything below narrows DOMPurify's defaults. The allow-lists themselves stay
 * DOMPurify's own, because they track new elements and attributes as browsers
 * ship them and a hand-written list would silently rot.
 */
const SANITIZE_CONFIG: Config = {
	// MathML and SVG are both required: KaTeX emits MathML for screen readers
	// and inline SVG for stretchy delimiters. Restricting to the html profile
	// alone renders every formula as unstyled fragments.
	USE_PROFILES: { html: true, mathMl: true, svg: true },

	// Markdown and KaTeX produce none of these, so anything here is either broken
	// output or hostile output.
	//
	// `style` needs no script to do damage: one rule can hide the surrounding UI
	// or paint a convincing overlay over the app. KaTeX is unaffected, since it
	// positions glyphs with style *attributes*, which stay allowed.
	//
	// The interactive controls are a credential-phishing surface — a form
	// rendered inside the app window looks exactly like the app asking — and no
	// chat reply has any reason to ask for input.
	//
	// The SVG entries are the long-standing filter-bypass vectors: foreignObject
	// smuggles HTML back in, `use` pulls in external references, and the
	// animation elements can retarget an attribute after sanitisation.
	FORBID_TAGS: [
		'style',
		'form',
		'input',
		'button',
		'textarea',
		'select',
		'option',
		'optgroup',
		'fieldset',
		'legend',
		'label',
		'output',
		'progress',
		'meter',
		'dialog',
		'foreignObject',
		'use',
		'animate',
		'animateMotion',
		'animateTransform',
		'set'
	],

	// Belt and braces: DOMPurify already strips event handlers, but these are
	// navigation and fetch triggers that do not look like handlers.
	FORBID_ATTR: ['formaction', 'action', 'ping', 'srcset', 'autofocus', 'target'],

	// Nothing in the pipeline emits data-* attributes, and they are the input to
	// most DOM-clobbering tricks.
	ALLOW_DATA_ATTR: false,

	// Prefix id/name so sanitised markup cannot shadow a property on document or
	// on a form — the clobbering defence DOMPurify ships but leaves off.
	SANITIZE_NAMED_PROPS: true,

	// Scheme allow-list. The default permits a broader set; a desktop app has no
	// use for anything but web links, mail links and in-message anchors, and
	// relative URLs mean nothing here because there is no server to resolve them.
	ALLOWED_URI_REGEXP: /^(?:https?:|mailto:|#)/i
};

function ensureMarkedKatex(): void {
	if (katexConfigured) return;
	katexConfigured = true;
	marked.use(
		markedKatex({
			throwOnError: false
		})
	);
}

/** Convert `\[...\]` / `\(...\)` to `$$...$$` / `$...$` for the KaTeX extension. */
export function preprocessChatMath(markdown: string): string {
	let processed = markdown.replace(/\\\[([\s\S]*?)\\\]/g, (_, math) => `$$${math}$$`);
	processed = processed.replace(/\\\(([\s\S]*?)\\\)/g, (_, math) => `$${math}$`);
	return processed;
}

function wrapPreAndTables(html: string): string {
	let out = html.replace(/<pre(\s[^>]*)?>/g, (match) => {
		if (match.includes('class=')) {
			return match.replace(/class="([^"]*)"/, 'class="$1 scrollable"');
		}
		return match.replace(/<pre/, '<pre class="scrollable"');
	});
	out = out.replace(/<table(\s[^>]*)?>/gi, '<div class="md-table-wrap scrollable"><table$1>');
	out = out.replace(/<\/table>/gi, '</table></div>');
	return out;
}

/**
 * Markdown → sanitized HTML for `{@html ...}`.
 * Must run synchronously during render (`$derived` / template); do not use `$effect`
 * for this, Svelte 5 can leave `{@html}` out of sync with post-render updates.
 */
export function parseChatMarkdownToHtmlSync(markdown: string): string {
	if (!markdown) return '';
	ensureMarkedKatex();
	try {
		const processed = preprocessChatMath(markdown);
		const result = marked.parse(processed, { async: false }) as string | Promise<string>;
		if (typeof result !== 'string') {
			return '';
		}
		// Sanitising last is what makes wrapPreAndTables safe to write as regexes:
		// whatever they mangle is still parsed and filtered before it reaches the DOM.
		return DOMPurify.sanitize(wrapPreAndTables(result), SANITIZE_CONFIG);
	} catch {
		return '';
	}
}
