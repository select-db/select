<script lang="ts">
	import Checkbox from '$lib/system/Checkbox/Checkbox.svelte';
	import type { graph } from '$lib/wailsjs/go/models';

	type Props = {
		databases: graph.DBInstanceNode[];
		dbOn: Record<string, boolean>;
		schemaOn: Record<string, boolean>;
		maxHeight: number;
	};

	let { databases, dbOn = $bindable(), schemaOn = $bindable(), maxHeight }: Props = $props();
</script>

<aside class="scope-panel" style:max-height="{maxHeight}px">
	<p class="scope-heading">Search in</p>
	<div class="scopes scrollable">
		{#each databases as db (db.id)}
			{@const schemas = (db.children ?? []).filter((c) => c.type === 'schema')}
			<div class="db-block">
				<div class="scope-row">
					<Checkbox
						checked={dbOn[db.id] !== false}
						size="sm"
						label={db.name}
						onchange={(on) => {
							dbOn = { ...dbOn, [db.id]: on };
						}}
					/>
				</div>
				{#if dbOn[db.id] !== false}
					{#each schemas as sch (sch.id)}
						<div class="scope-row schema-row">
							<Checkbox
								checked={schemaOn[sch.id] !== false}
								size="sm"
								label={sch.name}
								onchange={(on) => {
									schemaOn = { ...schemaOn, [sch.id]: on };
								}}
							/>
						</div>
					{/each}
				{/if}
			</div>
		{/each}
	</div>
</aside>

<style>
	.scope-panel {
		flex: 0 0 180px;
		min-width: 180px;
		z-index: 1;
		border-right: var(--border);
		box-sizing: border-box;
		background: var(--gray-100);

		display: flex;
		flex-direction: column;
	}

	.scope-heading {
		font-size: var(--fs-xs);
		color: var(--gray-800);
		text-transform: uppercase;
		letter-spacing: 0.05em;
		font-weight: 400;

		line-height: 39px;
		padding: 0 var(--space-sm-md);
		border-bottom: var(--border);
	}

	.scopes {
		flex-grow: 1;
		overflow-y: auto;
		padding: var(--space-xs-sm) var(--space-sm-md) 0 var(--space-sm-md);

		display: flex;
		flex-direction: column;
	}

	.scope-row {
		display: flex;
		align-items: center;
		min-height: 30px;
		margin: 0;
		padding: var(--spapce-xxs) 0 0 0;
	}

	.scope-row.schema-row {
		padding-left: var(--space-md);
		border-left: var(--border);
		margin-left: var(--space-xs-sm);
	}

	.scope-row :global(.checkbox-label) {
		font-family: 'JetBrains Mono', monospace;
		font-size: var(--fs-sm);
		font-weight: 200;
	}
</style>
