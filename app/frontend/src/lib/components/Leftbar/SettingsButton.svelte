<script lang="ts">
	import Button from '$lib/system/Button/Button.svelte';
	import { openSettingsSection } from '$lib/components/Layout/layoutStore';
</script>

<Button
	leftIcon="cog"
	iconSize={18}
	label="Settings"
	onclick={() => openSettingsSection()}
	size="sm"
	noLoader
/>
