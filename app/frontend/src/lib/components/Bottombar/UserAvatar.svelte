<script lang="ts">
	import { GetCurrentUserAvatar } from '$lib/wailsjs/go/user/User';
	import { onMount } from 'svelte';
	import { get } from 'svelte/store';
	import Contextable from '$lib/system/ContextMenu/Contextable.svelte';
	import type { ContextMenuOption } from '$lib/system/ContextMenu/types';
	import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
	import { Logout } from '$lib/wailsjs/go/system/System';
	import { tryCatch } from '$lib/utils/tryCatch';

	let avatarSrc: string = '';
	let userName: string = '';

	const loadAvatar = async () => {
		const ws = get(workspaceGraphStore);
		if (!ws?.user?.id) return;

		userName = ws?.user.name;
		const [r, err] = await tryCatch(GetCurrentUserAvatar, ws?.user.id);
		if (err) return;

		avatarSrc = r;
	};

	$: {
		void $workspaceGraphStore;
		loadAvatar();
	}

	onMount(() => loadAvatar());

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
