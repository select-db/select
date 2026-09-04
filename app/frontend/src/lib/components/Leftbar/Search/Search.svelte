<script lang="ts">
	import Button from '$lib/system/Button/Button.svelte';
	import { modalStore } from '$lib/system/Modal/ModalStore';
	import SearchModalContent from './SearchModalContent.svelte';
	import type { Component } from 'svelte';
</script>

<div class="search">
	<Button
		leftIcon="search"
		iconSize={15}
		onclick={() =>
			modalStore.set({
				content: () => SearchModalContent as unknown as Component,
				width: 650
			})}
		size="sm"
	/>
</div>
