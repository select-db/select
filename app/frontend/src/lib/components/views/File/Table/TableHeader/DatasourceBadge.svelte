<script lang="ts">
	import type { ContextMenuOption } from '$lib/system/ContextMenu/types';
	import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
	import Contextable from '$lib/system/ContextMenu/Contextable.svelte';
	import DatasourceIndicator from '$lib/components/shared/DatasourceIndicator/DatasourceIndicator.svelte';

	type Props = {
		datasourceId: string;
		run: (type: 'run' | 'explain' | 'plan', datasourceIds?: string[]) => Promise<void>;
		active?: boolean;
		error?: boolean;
		onclick?: (e: MouseEvent) => void;
	};

	let { datasourceId, run, active = false, error = false, onclick }: Props = $props();

	const datasources = $derived($workspaceGraphStore?.datasources ?? []);
	const name = $derived(datasources.find((dbi) => dbi.id === datasourceId)?.name ?? datasourceId);

	const options = $derived<ContextMenuOption[]>([
		{
			label: 'Run',
			icon: 'play',
			action: async (onClose: () => void) => {
				await run('run', [datasourceId]);
				onClose();
			}
		},
		{
			label: 'Plan (no execution)',
			icon: 'map',
			action: async (onClose: () => void) => {
				await run('plan', [datasourceId]);
				onClose();
			}
		},
		{
			label: 'Analyse (execute)',
			icon: 'chart',
			action: async (onClose: () => void) => {
				await run('explain', [datasourceId]);
				onClose();
			}
		}
	]);
</script>

<Contextable {options} direction="right">
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div class="db-badge" class:active {onclick}>
		<DatasourceIndicator id={datasourceId} size={17} loaderSize={15} {error} />
		<p style="margin-top: -1px;">{name}</p>
	</div>
</Contextable>

<style>
	.db-badge {
		height: 28px;
		display: flex;
		align-items: center;
		gap: var(--space-xs);

		padding: 0 var(--space-sm) 0 var(--space-xs-sm);
		border-radius: var(--br-sm);
		border: var(--bw) transparent solid;

		transition: border-color 0.15s ease-out;
	}
	.db-badge p {
		color: var(--gray-800);
	}
	.db-badge.active,
	.db-badge:hover {
		background: var(--gray-0);
		border-color: var(--border-color);
	}
	.db-badge.active p,
	.db-badge:hover p {
		color: var(--gray-1000);
	}
</style>
