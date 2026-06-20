<script lang="ts">
	import { isRightbarOpened, toggleSearchPanel } from '$lib/components/Rightbar/rightbarStore';
	import Button from '$lib/system/Button/Button.svelte';
</script>

<Button
	leftIcon="search"
	iconSize={16}
	size="sm"
	emphasis={$isRightbarOpened ? 'high' : 'low'}
	active={$isRightbarOpened}
	onclick={toggleSearchPanel}
	noBounce
/>
