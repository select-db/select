<script lang="ts">
	import ModalBody from '$lib/system/Modal/ModalBody.svelte';
	import ModalHeader from '$lib/system/Modal/ModalHeader.svelte';

	import { databaseFieldHelpContent, type DatabaseFieldKey } from './fieldHelpContent';

	type DatabaseFieldHelpModalProps = {
		field: DatabaseFieldKey;
	};

	let { field }: DatabaseFieldHelpModalProps = $props();

	const help = $derived(databaseFieldHelpContent[field]);
</script>

<ModalHeader icon="info" title={help.title} />

<ModalBody>
	<div class="section prose">
		<p class="section-title">What it is</p>
		<p class="section-text">{help.what}</p>
	</div>

	<div class="section prose">
		<p class="section-title">Where to find it</p>
		<p class="section-text">{help.where}</p>
	</div>

	{#if help.command}
		<div class="section">
			<div class="command-block selectable">
				<code>{help.command}</code>
			</div>
		</div>
	{/if}

	{#if help.table}
		<div class="section selectable">
			<p class="section-title">Quick reference</p>
			<div class="table-wrap">
				<table class="help-table">
					<thead>
						<tr>
							<th>{help.table.headers[0]}</th>
							<th>{help.table.headers[1]}</th>
						</tr>
					</thead>
					<tbody>
						{#each help.table.rows as [colA, colB], i (i)}
							<tr>
								<td><code class="inline-code">{colA}</code></td>
								<td>{colB}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		</div>
	{/if}
</ModalBody>

<style>
	.section {
		margin-bottom: var(--space-md);
	}

	.section:first-child {
		margin-top: 0;
	}

	.section + .section {
		margin-top: var(--space-md);
	}

	.section-title {
		margin: 0 0 var(--space-xs) 0;
		text-transform: uppercase;
		color: var(--gray-800);
	}

	.prose .section-text {
		max-width: 65ch;
		line-height: 1.55;
	}

	.section-text {
		margin: 0;
		font-size: var(--fs-sm);
		color: var(--gray-1000);
		white-space: pre-wrap;
	}

	.table-wrap {
		overflow-x: auto;
		border: var(--border);
		border-radius: var(--br-xs, 4px);
	}

	.help-table {
		width: 100%;
		border-collapse: collapse;
		font-size: var(--fs-sm);
	}

	.help-table th,
	.help-table td {
		padding: var(--space-xs) var(--space-sm);
		text-align: left;
		border-bottom: var(--border);
		vertical-align: top;
	}

	.help-table th {
		font-weight: 600;
		color: var(--gray-900);
		background-color: var(--gray-100);
	}

	.help-table th:last-child,
	.help-table td:last-child {
		border-left: var(--border);
	}

	.help-table tr:last-child td {
		border-bottom: none;
	}

	.command-block {
		display: flex;
		align-items: center;
		gap: var(--space-sm);
		padding: var(--space-xs) var(--space-sm);
		background-color: var(--gray-200);
		border: var(--border);
		border-radius: var(--br-xs);
	}

	.command-block code {
		flex: 1;
		font-family: ui-monospace, monospace;
		font-size: var(--fs-sm);
		color: var(--gray-1000);
		user-select: all;
		-webkit-user-select: all;
	}

	.inline-code {
		font-family: ui-monospace, monospace;
		background-color: var(--gray-100);
		color: var(--gray-900);
		border-radius: var(--br-xs);
	}

	:global(.examples-list p) {
		color: var(--gray-900);
		margin: var(--space-sm) 0;
	}
	:global(.examples-list code) {
		color: var(--red);
		background-color: var(--gray-300);
		padding: var(--space-xs) var(--space-sm);
		border-radius: var(--br-xs);
		font-style: italic;
	}
</style>
