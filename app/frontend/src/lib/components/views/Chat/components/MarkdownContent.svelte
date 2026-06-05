<script lang="ts">
	import 'highlight.js/styles/github.min.css';
	import 'katex/dist/katex.min.css';
	import { parseChatMarkdownToHtmlSync } from './chatMarkdown';

	type Props = {
		content: string;
	};

	let { content }: Props = $props();

	const safeHtml = $derived.by(() => parseChatMarkdownToHtmlSync(content));
</script>

<div class="selectable markdown-wrapper">
	<!-- eslint-disable-next-line svelte/no-at-html-tags -->
	{@html safeHtml}
</div>

<style>
	.markdown-wrapper {
		display: flex;
		flex-direction: column;
		gap: var(--space-sm-md);
		white-space: pre-wrap;
		word-break: break-word;
	}

	.markdown-wrapper,
	.markdown-wrapper :global(p),
	.markdown-wrapper :global(pre),
	.markdown-wrapper :global(ul),
	.markdown-wrapper :global(ol),
	.markdown-wrapper :global(h1),
	.markdown-wrapper :global(h2),
	.markdown-wrapper :global(h3) {
		margin: 0;
		color: var(--gray-900);
		font-size: var(--fs-md);
		line-height: 20px;
	}

	.markdown-wrapper :global(ul),
	.markdown-wrapper :global(ol) {
		display: flex;
		flex-direction: column;
		gap: var(--space-sm);
		padding-left: 1.5em;
		white-space: pre-wrap;
	}

	.markdown-wrapper :global(h1),
	.markdown-wrapper :global(h2),
	.markdown-wrapper :global(h3) {
		font-weight: var(--fw-bold);
		white-space: pre-wrap;
	}

	.markdown-wrapper :global(p) {
		white-space: pre-wrap;
	}

	.markdown-wrapper :global(pre) {
		padding: var(--space-sm);
		background-color: var(--gray-400);
		border-radius: var(--br-xs);
		border: var(--border);
		height: fit-content;
		overflow-x: auto;
		overflow-y: hidden;
		font-size: var(--fs-sm);
	}

	.markdown-wrapper :global(code) {
		font-family: ui-monospace, monospace;
		height: fit-content;
		color: var(--red);
		background-color: var(--gray-400);
		padding: 1px var(--space-xs);
		border: var(--border);
		border-radius: var(--br-xs);
		font-size: var(--fs-sm);
		white-space: pre-wrap;
	}

	.markdown-wrapper :global(pre code) {
		padding: 0;
		background: none;
		color: var(--gray-1000);
		background-color: none;
		padding: none;
		border: none;
		border-radius: none;
	}

	/* KaTeX math styling */
	.markdown-wrapper :global(.katex) {
		color: var(--gray-900);
		font-size: 1em;
	}

	.markdown-wrapper :global(.katex-display) {
		margin: var(--space-sm) 0;
		overflow-x: auto;
		overflow-y: hidden;
	}

	.markdown-wrapper :global(.katex-display > .katex) {
		display: inline-block;
		text-align: initial;
	}

	/* GFM tables, .scrollable = app.css scrollbar. Reset pre-wrap/line-height so
	   stray whitespace/newlines between div and <table> (from marked HTML) don’t
	   leave a sliver of wrapper background at top/left. */
	.markdown-wrapper :global(.md-table-wrap.scrollable) {
		max-width: 100%;
		overflow-x: auto;
		overflow-y: hidden;
		margin: var(--space-xs) 0;
		border-radius: var(--br-xs);
		background: var(--gray-0);
		border: var(--border);
		padding: 0;
		white-space: normal;
		word-break: normal;
		line-height: 0;
	}

	.markdown-wrapper :global(.md-table-wrap.scrollable table) {
		width: max-content;
		min-width: 100%;
		border-collapse: collapse;
		border-spacing: 0;
		margin: 0;
		font-size: var(--fs-sm);
		line-height: 1.35;
		color: var(--gray-900);
	}

	.markdown-wrapper :global(.md-table-wrap.scrollable th),
	.markdown-wrapper :global(.md-table-wrap.scrollable td) {
		border-right: var(--border);
		border-bottom: var(--border);
		padding: var(--space-xs) var(--space-sm);
		text-align: left;
		vertical-align: top;
	}

	.markdown-wrapper :global(.md-table-wrap.scrollable th:last-child),
	.markdown-wrapper :global(.md-table-wrap.scrollable td:last-child) {
		border-right: none;
	}

	.markdown-wrapper :global(.md-table-wrap.scrollable tr:last-child th),
	.markdown-wrapper :global(.md-table-wrap.scrollable tr:last-child td) {
		border-bottom: none;
	}

	.markdown-wrapper :global(.md-table-wrap.scrollable th) {
		background: var(--gray-550);
		font-weight: var(--fw-bold);
		white-space: nowrap;
	}

	.markdown-wrapper :global(.md-table-wrap.scrollable tbody tr:nth-child(even)) {
		background: var(--gray-400);
	}

	.markdown-wrapper :global(.md-table-wrap.scrollable tbody tr:hover) {
		background: var(--gray-550);
	}
</style>
