<script lang="ts">
	import Contextable from '$lib/system/ContextMenu/Contextable.svelte';
	import type { ContextMenuOption } from '$lib/system/ContextMenu/types';
	import { Logout } from '$lib/bindings/selectDb/internal/system/system';
	import { currentUserStore } from '$lib/stores/currentUserStore';

	// The signed-in user rather than a workspace member, since this renders with
	// no folder open.
	const user = $derived($currentUserStore);

	const options: ContextMenuOption[] = [
		{
			label: 'Log out',
			action: async (onclose) => {
				await Logout();
				onclose();
			}
		}
	];
</script>

{#if user}
	<Contextable
		{options}
		direction="right"
		on="click"
		anchor="child"
		style="display:flex; align-items: stretch"
	>
		<div class="wrapper avatar" data-test="bottombar.user" data-test-value={user.name}>
			{#if user.avatar}
				<img src={user.avatar} alt="User Avatar" />
			{/if}
			<p>{user.name}</p>
		</div>
	</Contextable>
{/if}

<style>
	.wrapper {
		display: flex;
		align-items: center;
		padding: var(--space-xs-sm) var(--space-sm);
		border-radius: var(--br-xs);
	}
	.wrapper:hover {
		background-color: var(--gray-300);
	}
	.wrapper p {
		color: var(--gray-800);
	}
	.wrapper:hover p {
		color: var(--gray-1000);
	}
	:global(.wrapper.avatar svg) {
		stroke: var(--gray-900);
	}
	:global(.wrapper.avatar:hover svg) {
		stroke: var(--gray-1000);
	}
	img {
		width: 14px;
		border-radius: var(--br-sm);
		margin-right: var(--space-sm);
	}
</style>
