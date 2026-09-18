<script lang="ts">
	import Menu from '$lib/system/Menu/Menu.svelte';
	import type { MenuOption } from '$lib/system/Menu/Menu.types';
	import { modalStore } from '$lib/system/Modal/ModalStore';
	import { notify } from '$lib/system/Notifications/notificationsStore';
	import { AlertType } from '$lib/system/Alert/types';
	import { tryCatch } from '$lib/utils/tryCatch';
	import {
		ListServers,
		GetCurrentServer,
		SetCurrentServer,
		CreateServer,
		RemoveServer
	} from '$lib/bindings/selectDb/internal/server/server';
	import type { Component } from 'svelte';
	import ConfirmRemoveServerModal from './ConfirmRemoveServerModal.svelte';

	let servers = $state<string[]>([]);
	let currentServer = $state('');
	let loading = $state(true);
	let searchQuery = $state('');

	async function load() {
		loading = true;
		const [list, err] = await tryCatch(ListServers);
		const [current, errCur] = await tryCatch(GetCurrentServer);
		loading = false;
		if (err) {
			notify({ type: AlertType.Error, message: err?.message ?? 'Failed to load servers' });
			return;
		}
		servers = list ?? [];
		currentServer = errCur ? '' : (current ?? '');
	}

	async function switchTo(domain: string) {
		if (domain === currentServer) return;
		const [, err] = await tryCatch(SetCurrentServer, domain);
		if (err) {
			notify({ type: AlertType.Error, message: err?.message ?? 'Failed to switch server' });
			return;
		}
		currentServer = domain;
		modalStore.set(null);
	}

	async function createAndSwitch() {
		const domain = searchQuery.trim();
		if (!domain) return;
		const [, errCreate] = await tryCatch(CreateServer, domain);
		if (errCreate) {
			notify({ type: AlertType.Error, message: errCreate?.message ?? 'Failed to create server' });
			return;
		}
		const [, errSet] = await tryCatch(SetCurrentServer, domain);
		if (errSet) {
			notify({ type: AlertType.Error, message: errSet?.message ?? 'Failed to set server' });
			return;
		}
		currentServer = domain;
		modalStore.set(null);
	}

	function openRemoveConfirm(domain: string) {
		modalStore.set({
			content: (() => ConfirmRemoveServerModal) as () => Component,
			props: {
				domain,
				onClose: () => modalStore.set(null),
				onConfirm: async () => {
					const [, err] = await tryCatch(RemoveServer, domain);
					if (err) {
						notify({ type: AlertType.Error, message: err?.message ?? 'Failed to remove server' });
						return;
					}
					modalStore.set(null);
				}
			},
			width: 420
		});
	}

	const menuOptions = $derived.by((): MenuOption<string>[] => {
		const options: MenuOption<string>[] = [];

		for (const domain of servers) {
			options.push({
				id: domain,
				label: domain,
				badge: domain === currentServer ? 'current' : undefined,
				action: async () => switchTo(domain)
			});
			if (domain !== currentServer) {
				options.push({
					id: `remove-${domain}`,
					label: `Remove ${domain}`,
					icon: 'minus',
					action: () => openRemoveConfirm(domain)
				});
			}
		}

		const trimmedQuery = searchQuery.trim();
		const exactMatch = trimmedQuery && servers.some((s) => s.toLowerCase() === trimmedQuery.toLowerCase());
		const showCreateOption = trimmedQuery && !exactMatch;

		if (showCreateOption) {
			options.push({
				id: '__create__',
				label: `Add server '${trimmedQuery}'`,
				icon: 'plus',
				action: createAndSwitch
			});
		}

		return options;
	});

	$effect(() => {
		load();
	});
</script>

<Menu
	{loading}
	bind:searchQuery
	options={menuOptions}
	searchEnabled
	searchPlaceholder="Domain (e.g. api.select-db.com)..."
	emptyMessage="No servers. Enter a domain above to add one."
	noResultsMessage={`No server "${searchQuery}"`}
	onClose={() => modalStore.set(null)}
	maxHeight={400}
	width={400}
	noBorder
/>
