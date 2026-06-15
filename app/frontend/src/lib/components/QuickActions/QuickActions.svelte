<script lang="ts">
	import Button from '$lib/system/Button/Button.svelte';
	import { quickActions, executeQuickAction } from './quickActionsData';

	const iconByActionId: Record<string, 'file' | 'schema' | 'chat' | 'terminal' | 'cog'> = {
		'quick-action-new-query': 'file',
		'quick-action-open-schema': 'schema',
		'quick-action-open-chat': 'chat',
		'quick-action-open-terminal': 'terminal',
		'quick-action-open-settings': 'cog'
	};
</script>

<div class="quick-actions">
	{#each quickActions as action (action.id)}
		<Button
			content={action.label}
			leftIcon={iconByActionId[action.id]}
			iconSize={18}
			emphasis="low"
			size="md"
			onclick={() => executeQuickAction(action)}
		/>
	{/each}
</div>

<style>
	.quick-actions {
		display: flex;
		flex-direction: column;
		gap: var(--space-xs);
		padding-left: var(--space-sm);
	}
</style>
