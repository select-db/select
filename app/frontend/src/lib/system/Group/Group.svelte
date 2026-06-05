<script lang="ts">
	export let style: string = '';
	export let className: string = '';
</script>

<div class={`group ${className}`} {style}>
	<slot />
</div>

<style>
	.group {
		display: flex;
		gap: var(--space-xs);
		align-items: center;
	}
</style>
