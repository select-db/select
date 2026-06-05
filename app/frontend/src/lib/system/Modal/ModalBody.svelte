<script lang="ts">
	export let style: string = '';
</script>

<div class="body scrollable" {style}>
	<slot />
</div>

<style>
	.body {
		padding: var(--space-sm);
		background-color: var(--gray-0);
	}
</style>
