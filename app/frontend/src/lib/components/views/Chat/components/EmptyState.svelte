<div class="wrapper">
	<p class="empty selectable">
		Set at least one API key in your workspace <code>.env</code> file.
	</p>

	<table class="providers selectable">
		<thead>
			<tr>
				<th>Provider</th>
				<th><code>.env</code> variable name</th>
			</tr>
		</thead>
		<tbody>
			<tr>
				<td>Claude</td>
				<td><code>ANTHROPIC_API_KEY</code></td>
			</tr>
			<tr>
				<td>OpenAI</td>
				<td><code>OPENAI_API_KEY</code></td>
			</tr>
			<tr>
				<td>Google Gemini</td>
				<td><code>GEMINI_API_KEY</code></td>
			</tr>
			<tr>
				<td>OpenRouter</td>
				<td><code>OPENROUTER_API_KEY</code></td>
			</tr>
			<tr>
				<td>Grok</td>
				<td><code>XAI_API_KEY</code></td>
			</tr>
		</tbody>
	</table>
</div>

<style>
	.wrapper {
		display: flex;
		flex-direction: column;
		gap: var(--space-sm);
		padding: var(--space-md);
	}
	.empty {
		color: var(--gray-900);
		font-size: var(--fs-sm);
	}

	.providers {
		width: 100%;
		table-layout: fixed;
		border-collapse: collapse;
		font-size: var(--fs-sm);
		color: var(--gray-900);
	}

	.providers th,
	.providers td {
		padding: var(--space-sm) 0;
		padding-right: var(--space-md);
		font-weight: 300;
		line-height: 2;
		vertical-align: middle;
		white-space: nowrap;
	}

	.providers th:first-child,
	.providers td:first-child {
		width: 10rem;
		white-space: nowrap;
		padding-right: var(--space-lg);
	}

	.providers th:last-child,
	.providers td:last-child {
		width: auto;
		text-align: left;
	}

	.providers th {
		text-align: left;
		color: var(--gray-900);
		border-bottom: var(--border);
	}

	.providers td code {
		vertical-align: middle;
		line-height: 1.5;
	}

	code {
		font-family: ui-monospace, monospace;
		height: fit-content;
		color: var(--gray-800);
		background-color: var(--gray-400);
		padding: 1px var(--space-xs);
		border: var(--border);
		border-radius: var(--br-xs);
	}
</style>
