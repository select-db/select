<script lang="ts">
	import { GetCurrentUser, GetCurrentUserAvatar } from '$lib/bindings/selectDb/internal/user/user';
	import { onMount } from 'svelte';
	import Contextable from '$lib/system/ContextMenu/Contextable.svelte';
	import type { ContextMenuOption } from '$lib/system/ContextMenu/types';
	import { Logout } from '$lib/bindings/selectDb/internal/system/system';
	import { tryCatch } from '$lib/utils/tryCatch';

	let avatarSrc: string = '';
	let userName: string = '';

	// The signed-in user, not the workspace's: this shows with no folder open.
	const loadUser = async () => {
		const [user, err] = await tryCatch(GetCurrentUser);
		if (err || !user?.id) return;

		userName = user.name ?? '';
		const [avatar, avatarErr] = await tryCatch(GetCurrentUserAvatar, user.id);
		if (avatarErr) return;

		avatarSrc = avatar;
	};

	onMount(() => loadUser());

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

<Contextable
	{options}
	direction="right"
	on="click"
	anchor="child"
	style="display:flex; align-items: stretch"
>
	<div class="wrapper avatar">
		{#if avatarSrc}
			<img src={avatarSrc} alt="User Avatar" />
		{/if}
		<p>{userName}</p>
	</div>
</Contextable>

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
