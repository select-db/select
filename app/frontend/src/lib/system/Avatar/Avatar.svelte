<script lang="ts">
	type Props = {
		name?: string | null;
		src?: string | null;
		size?: number;
	};

	let { name, src, size = 18 }: Props = $props();

	function initials(n: string | null | undefined): string {
		if (!n) return '?';
		return n
			.split(/\s+/)
			.slice(0, 2)
			.map((w) => w[0]?.toUpperCase() ?? '')
			.join('');
	}
</script>

<span class="avatar" style:width="{size}px" style:height="{size}px" style:font-size="{Math.round(size * 0.44)}px">
	{#if src}
		<img {src} alt={name ?? ''} />
	{:else}
		{initials(name)}
	{/if}
</span>

<style>
	.avatar {
		border-radius: 50%;
		background: var(--gray-200);
		color: var(--gray-900);
		font-weight: 600;
		display: flex;
		align-items: center;
		justify-content: center;
		overflow: hidden;
		flex-shrink: 0;
		letter-spacing: 0;
		user-select: none;
	}

	img {
		width: 100%;
		height: 100%;
		object-fit: cover;
	}
</style>
