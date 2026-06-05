import { marked } from 'marked';
import markedKatex from 'marked-katex-extension';
import DOMPurify from 'dompurify';

let katexConfigured = false;

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
		return DOMPurify.sanitize(wrapPreAndTables(result));
	} catch {
		return '';
	}
}
