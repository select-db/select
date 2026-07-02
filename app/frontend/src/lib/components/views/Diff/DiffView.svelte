<script lang="ts">
	import { updateTab, type Tab } from '$lib/components/Layout/layoutStore';
	import { getDiffApproval } from '$lib/components/views/Chat/tools/diffApprovals';
	import Editor from '$lib/components/views/File/Editor/Editor.svelte';
	import SegmentedControl from '$lib/system/SegmentedControl/SegmentedControl.svelte';
	import Icon from '$lib/system/Icon/Icon.svelte';
	import Button from '$lib/system/Button/Button.svelte';

	type Props = {
		tab: Tab;
	};

	let { tab }: Props = $props();

	const diff = $derived(tab.diff);
	const labelLeft = $derived((diff?.panels[0]?.meta?.label as string) ?? 'Original');
	const labelRight = $derived((diff?.panels[1]?.meta?.label as string) ?? 'Modified');
	const isAgentDiff = $derived(
		diff?.context === 'agent' &&
			diff?.meta &&
			typeof (diff.meta as { uri?: string }).uri === 'string'
	);
	function setViewMode(mode: 'inline' | 'side-by-side') {
		if (!diff) return;
		updateTab({
			...tab,
			diff: { ...diff, viewMode: mode }
		});
	}

	function handleDiffChange(content: string, panelIndex?: number) {
		if (!diff || panelIndex == null || panelIndex < 0 || panelIndex >= diff.panels.length) return;
		const panels = [...diff.panels];
		panels[panelIndex] = { ...panels[panelIndex], content };
		updateTab({
			...tab,
			diff: { ...diff, panels }
		});
	}

	async function handleAllow() {
		if (!isAgentDiff) return;
		const toolCallId = (diff?.meta as { toolCallId?: string })?.toolCallId;
		if (toolCallId) await getDiffApproval(toolCallId)?.approve();
	}

	async function handleDeny() {
		if (!isAgentDiff) return;
		const toolCallId = (diff?.meta as { toolCallId?: string })?.toolCallId;
		if (toolCallId) {
			await getDiffApproval(toolCallId)?.deny();
		}
	}
</script>

{#if diff}
	<div class="diff-view">
		<div class="header">
			<div class="left-wrapper">
				<SegmentedControl
					options={[
						{ id: 'side-by-side', icon: 'column-2', tooltip: 'Side by side' },
						{ id: 'inline', icon: 'column-1', tooltip: 'Inline' }
					]}
					value={diff.viewMode}
					onSelect={(value) => setViewMode(value as 'inline' | 'side-by-side')}
				/>
				<div class="labels">
					<p>{labelLeft}</p>
					<Icon icon="diff" size={16} stroke="var(--gray-700)" />
					<p>{labelRight}</p>
				</div>
			</div>

			<div class="right-wrapper">
				{#if isAgentDiff}
					<Button content="Allow" emphasis="warning" size="sm" onclick={handleAllow} active />
					<Button content="Deny" emphasis="low" size="sm" onclick={handleDeny} />
				{/if}
			</div>
		</div>
		<div class="editor-wrapper">
			<Editor {tab} onchange={handleDiffChange} />
		</div>
	</div>
{/if}

<style>
	.diff-view {
		height: 100%;
		display: flex;
		flex-direction: column;
		background-color: var(--gray-100);
		overflow: hidden;
	}

	.header {
		display: flex;
		align-items: start;
		justify-content: space-between;
		gap: var(--space-sm);
		padding: var(--space-xs-sm) var(--space-sm) 0 var(--space-sm);
		border-bottom: var(--border);
		min-height: 35px;
	}

	.left-wrapper {
		display: flex;
		align-items: center;
		gap: var(--space-sm);
		padding-top: var(--space-xxs);
	}

	.right-wrapper {
		display: flex;
		align-items: stretch;
		gap: var(--space-xs-sm);
		margin-left: auto;
		height: 28px;
	}

	.labels {
		display: flex;
		gap: var(--space-xs);
	}

	.labels p {
		color: var(--gray-800);
	}

	.editor-wrapper {
		flex: 1;
		min-height: 0;
		overflow: hidden;
	}
</style>
