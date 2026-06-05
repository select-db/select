<script lang="ts">
	import type { TabGroup, SplitContainer } from './layoutStore';
	import GroupRenderer from './Renders/GroupRenderer.svelte';
	import SplitRenderer from './Renders/SplitRenderer.svelte';

	type Props = {
		node: TabGroup | SplitContainer;
	};

	let { node }: Props = $props();
</script>

{#if node == null}
	<!-- Guard: avoid passing undefined to child -->
{:else if 'type' in node && node.type === 'split'}
	<SplitRenderer split={node} />
{:else}
	<GroupRenderer group={node as TabGroup} />
{/if}
