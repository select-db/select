<script lang="ts">
	import { isLeftbarOpened, leftPanelTab, toggleLeftPanelTab } from '$lib/components/Leftbar/store';
	import { gitFileStatusStore } from '$lib/components/views/Git/gitStore';
	import Button from '$lib/system/Button/Button.svelte';

	const status = $derived($gitFileStatusStore);
	const uncommittedCount = $derived(
		status
			? (status.staged?.length ?? 0) +
					(status.unstaged?.length ?? 0) +
					(status.untracked?.length ?? 0)
			: 0
	);
	const ahead = $derived(status?.commitsAhead ?? 0);
	const behind = $derived(status?.commitsBehind ?? 0);
	const badgeParts: string[] = $derived([
		...(uncommittedCount > 0 ? [`~${uncommittedCount}`] : []),
		...(ahead > 0 || behind > 0 ? [`↑${ahead}`, `↓${behind}`] : [])
	]);
	const badge = $derived(badgeParts.length > 0 ? badgeParts.join(' ') : undefined);
</script>

<Button
	leftIcon="github-branch"
	iconSize={17}
	noRadius
	noBounce
	emphasis={$isLeftbarOpened && $leftPanelTab === 'github-branch' ? 'high' : 'low'}
	active={$isLeftbarOpened && $leftPanelTab === 'github-branch'}
	onclick={() => toggleLeftPanelTab('github-branch')}
	{badge}
/>
