<script lang="ts">
	import type { ToolCallPart } from '$lib/components/views/Chat/core';
	import Button from '$lib/system/Button/Button.svelte';
	import SqlViewer from '$lib/system/SqlViewer/SqlViewer.svelte';
	import { tryCatch } from '$lib/utils/tryCatch';
	import { highlightJson } from '../utils/formatting';
	import { highlightCommand } from '../utils/commandHighlight';
	import ToolCardTitle from './ToolCardTitle.svelte';

	function getStatementFromArgs(argumentsJson: string) {
		const [parsed, parseErr] = tryCatch(JSON.parse, argumentsJson.trim());
		if (parseErr || parsed == null) {
			return null;
		}

		const s = (parsed as { statement?: string }).statement;
		return typeof s === 'string' && s.length > 0 ? s : null;
	}

	function getCommandFromArgs(argumentsJson: string): { command: string; args: string[] } | null {
		const [parsed, parseErr] = tryCatch(JSON.parse, argumentsJson.trim());
		if (parseErr || parsed == null) return null;

		const p = parsed as { command?: string; args?: string[] };
		if (typeof p.command !== 'string' || !p.command) return null;

		return {
			command: p.command,
			args: Array.isArray(p.args) ? p.args : []
		};
	}

	type Props = {
		toolCall: ToolCallPart;
		expanded: boolean;
		onToggle: () => void;
		onApprove: () => void;
		onDeny: () => void;
	};

	let { toolCall, expanded, onToggle, onApprove, onDeny }: Props = $props();

	const pending = $derived(toolCall.state === 'approval-requested');
	const failed = $derived(
		toolCall.output != null && (toolCall.output as { success?: boolean }).success === false
	);
	const showBody = $derived(expanded || pending);
	const statusLabel = $derived(
		pending ? 'Awaiting approval' : failed ? '✕' : toolCall.output != null ? '✓' : '…'
	);
	const isSqlTool = $derived(
		['execute_query', 'execute_statement', 'plan_query', 'explain_query'].includes(toolCall.name)
	);
	const sqlStatement = $derived(getStatementFromArgs(toolCall.arguments));
	const isCommandTool = $derived(toolCall.name === 'execute_command');
	const commandInfo = $derived(getCommandFromArgs(toolCall.arguments));
	const highlightedCommand = $derived(
		commandInfo ? highlightCommand(commandInfo.command, commandInfo.args) : ''
	);
</script>

<div class="tool-card" class:pending>
	<button type="button" class="tool-card-header" onclick={onToggle} aria-expanded={expanded}>
		<ToolCardTitle {toolCall} />
		<span class="tool-state" class:success={!failed && toolCall.output != null} class:failed>
			{statusLabel}
		</span>
	</button>
	{#if isSqlTool && sqlStatement != null}
		<div class="tool-card-sql">
			<SqlViewer sql={sqlStatement!} className="surface-light" format />
		</div>
	{/if}
	{#if isCommandTool && commandInfo != null}
		<div class="tool-card-command">
			<code class="command-line">{highlightedCommand}</code>
		</div>
	{/if}
	{#if showBody}
		<div class="tool-card-body">
			{#if !isSqlTool && !isCommandTool}
				<div class="tool-section">
					<span class="tool-label">Input</span>
					<pre class="tool-pre tool-json"><code class="hljs scrollable selectable"
							>{highlightJson(toolCall.arguments)}</code
						></pre>
				</div>
			{/if}
			{#if pending}
				<div class="tool-actions">
					<Button emphasis="warning" size="sm" content="Allow" onclick={onApprove} active />
					<Button content="Deny" emphasis="low" size="sm" onclick={onDeny} />
				</div>
			{/if}
			{#if toolCall.output != null}
				<div class="tool-section">
					<span class="tool-label">Output</span>
					<pre class="tool-pre tool-json"><code class="hljs scrollable selectable"
							>{highlightJson(toolCall.output)}</code
						></pre>
				</div>
			{/if}
		</div>
	{/if}
</div>

<style>
	.tool-card {
		border: var(--border);
		border-radius: var(--br-xs);
		background-color: var(--gray-0);
		overflow: hidden;
		font-size: var(--fs-sm);
	}

	.tool-card.pending {
		border-color: var(--orange);
	}

	.tool-card-header {
		display: flex;
		align-items: center;
		gap: var(--space-sm);
		width: 100%;
		padding: var(--space-sm) var(--space-sm-md);
		background: none;
		border: none;
		font: inherit;
		color: inherit;
		text-align: left;
		outline: none;
	}

	.tool-card-header:hover {
		background-color: var(--gray-400);
	}

	.tool-card-sql {
		border-top: var(--border);
		min-height: 60px;
		max-height: 130px;
		min-width: 360px;
		position: relative;
	}

	.tool-card-sql :global(.editorContainer) {
		min-height: 60px;
		height: 130px;
	}

	.tool-card-command {
		border-top: var(--border);
		padding: var(--space-sm) var(--space-sm-md);
		background-color: var(--gray-400);
		overflow-x: auto;
	}

	.command-line {
		font-family: ui-monospace, 'SF Mono', Menlo, Monaco, monospace;
		font-size: var(--fs-sm);
		white-space: pre-wrap;
		word-break: break-word;
	}

	.tool-state {
		margin-left: auto;
		opacity: 0.7;
	}

	.tool-state.success {
		color: var(--green);
	}

	.tool-state.failed {
		color: var(--red);
		opacity: 1;
	}

	.tool-card-body {
		display: flex;
		flex-direction: column;
		gap: var(--space-sm);
		padding: var(--space-sm-md);
		border-top: var(--border);
	}

	.tool-section {
		display: flex;
		flex-direction: column;
		gap: var(--space-xs);
	}

	.tool-label {
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--gray-800);
	}

	.tool-pre {
		font-family: ui-monospace, 'SF Mono', Menlo, Monaco, monospace;
		font-size: 0.85em;
		line-height: 1.5;
		white-space: pre;
		word-break: break-word;
		background-color: var(--gray-400);
		border-radius: var(--br-xs);
	}

	.tool-pre.tool-json {
		white-space: pre-wrap;
		tab-size: 2;
	}

	.tool-pre.tool-json code,
	.tool-pre.tool-json code.hljs {
		font-family: inherit;
		font-size: inherit;
		background: transparent !important;
		border: var(--border);
	}

	.tool-pre.tool-json code.hljs {
		padding: var(--space-sm);
	}

	.tool-actions {
		display: flex;
		gap: var(--space-xs);
	}
</style>
