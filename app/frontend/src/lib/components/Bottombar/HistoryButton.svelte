<script lang="ts">
	import {
		isRightbarOpened,
		rightPanelTab,
		togglePanelTab
	} from '$lib/components/Rightbar/rightbarStore';
	import Button from '$lib/system/Button/Button.svelte';

	const active = $derived($isRightbarOpened && $rightPanelTab === 'history');
</script>

<Button
	leftIcon="clock"
	iconSize={16}
	size="sm"
	emphasis={active ? 'high' : 'low'}
	{active}
	onclick={() => togglePanelTab('history')}
	noBounce
/>
