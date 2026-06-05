<script lang="ts">
	import Button from '$lib/system/Button/Button.svelte';

	type Props = {
		onclick?: () => void;
	};

	let { onclick }: Props = $props();
</script>

<Button leftIcon="text" iconSize={17} label="Format" {onclick} />
