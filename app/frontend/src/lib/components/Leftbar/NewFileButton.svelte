<script lang="ts">
	import Button from '$lib/system/Button/Button.svelte';
	import { executeCommand } from '$lib/stores/commandRegistry';
</script>

<Button
	leftIcon="plus"
	iconSize={19}
	label="New SQL file"
	onclick={() => {
		executeCommand('workbench.newSqlFile');
	}}
	size="sm"
	emphasis="high"
	noLoader
/>
