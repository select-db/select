<script lang="ts">
	import {
		isRightbarOpened,
		rightPanelTab,
		togglePanelTab
	} from '$lib/components/Rightbar/rightbarStore';
	import Button from '$lib/system/Button/Button.svelte';

	const active = $derived($isRightbarOpened && $rightPanelTab === 'search');
</script>

<Button
	leftIcon="search"
	iconSize={16}
	size="sm"
	emphasis={active ? 'high' : 'low'}
	{active}
	onclick={() => togglePanelTab('search')}
	noBounce
/>
