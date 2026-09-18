<script lang="ts">
	import { isLeftbarOpened, leftPanelTab, toggleLeftPanelTab } from '$lib/components/Leftbar/store';
	import Button from '$lib/system/Button/Button.svelte';
</script>

<Button
	leftIcon="files"
	label="Files"
	iconSize={17}
	size="sm"
	emphasis={$isLeftbarOpened && $leftPanelTab === 'files' ? 'high' : 'low'}
	active={$isLeftbarOpened && $leftPanelTab === 'files'}
	onclick={() => toggleLeftPanelTab('files')}
	noBounce
/>
