<script lang="ts">
	import { onMount } from 'svelte';
	import Select from '$lib/system/Select/Select.svelte';
	import type { SelectOption } from '$lib/system/Select/Select.types';
	import LoginBtn from './LoginBtn.svelte';
	import ServerIndicator from '$lib/components/shared/ServerIndicator/ServerIndicator.svelte';
	import Alert from '$lib/system/Alert/Alert.svelte';
	import Button from '$lib/system/Button/Button.svelte';
	import { AlertType } from '$lib/system/Alert/types';
	import { modalStore } from '$lib/system/Modal/ModalStore';
	import { notify } from '$lib/system/Notifications/notificationsStore';
	import { tryCatch } from '$lib/utils/tryCatch';
	import {
		fetchServerManifests,
		serverIndicatorStore
	} from '$lib/components/shared/ServerIndicator/serverIndicatorStore';
	import {
		ListServers,
		GetCurrentServer,
		SetCurrentServer,
		CreateServer,
		RemoveServer,
		GetDefaultServer
	} from '$lib/bindings/selectDb/internal/server/server';
	import type { Component } from 'svelte';
	import ConfirmRemoveServerModal from './ConfirmRemoveServerModal.svelte';
	import { GetAppEnv } from '$lib/bindings/selectDb/internal/system/system';
	import RegionSelect from './RegionSelect.svelte';
	import Wordmark from '$lib/components/shared/Wordmark/Wordmark.svelte';

	let servers = $state<SelectOption[]>([]);
	let currentServer = $state('');
	let defaultServer = $state('');
	let loading = $state(true);
	let serverSelectOpen = $state(false);
	let appEnv = $state('');
	let showAdvanced = $state(false);
	const isProd = $derived(appEnv === 'production');

	$effect(() => {
		const list = servers.map((s) => s.value);
		if (list.length > 0) fetchServerManifests(list);
	});

	async function loadServers() {
		loading = true;
		const [list, err] = await tryCatch(ListServers);
		const [current, errCur] = await tryCatch(GetCurrentServer);
		loading = false;
		if (err) {
			notify({ type: AlertType.Error, message: err?.message ?? 'Failed to load servers' });
			return;
		}
		servers = (list ?? []).map((domain) => ({ value: domain, label: domain }));
		currentServer = errCur ? '' : (current ?? '');
	}

	async function onServerChange(domain: string) {
		const [, err] = await tryCatch(SetCurrentServer, domain);
		if (err) {
			notify({ type: AlertType.Error, message: err?.message ?? 'Failed to switch server' });
			// Revert UI to actual persisted server (binding may have already updated currentServer)
			const [actual] = await tryCatch(GetCurrentServer);
			if (typeof actual === 'string') currentServer = actual;
			return;
		}
		currentServer = domain;
	}

	// Region selection on prod: switch to the region's domain, then refresh the
	// list so its manifest/version is fetched and shown.
	async function onRegionSelect(domain: string) {
		await onServerChange(domain);
		await loadServers();
	}

	async function onCreateServer(domain: string) {
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
		await loadServers();
	}

	// Accepts: hostname, hostname:port, IP, IP:port. Rejects protocols, spaces, slashes.
	const domainRe = /^[\w.-]+(:\d{1,5})?$/;
	function validateDomain(q: string): true | string {
		return domainRe.test(q.trim())
			? true
			: 'Enter a valid domain (e.g. api.example.com or 192.168.1.1:8080)';
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
					await loadServers();
				}
			},
			width: 420
		});
	}

	function handleRemoveClick(e: MouseEvent, domain: string) {
		e.stopPropagation();
		e.preventDefault();
		serverSelectOpen = false;
		openRemoveConfirm(domain);
	}

	onMount(async () => {
		await loadServers();
		const [defaultDomain] = await tryCatch(GetDefaultServer);
		defaultServer = typeof defaultDomain === 'string' ? defaultDomain : '';
		const [env] = await tryCatch(GetAppEnv);
		appEnv = typeof env === 'string' ? env : '';
	});
</script>

<div class="wrapper">
	<div class="item"></div>
	<div class="item center bottom">
		<Wordmark height="1.5rem" color="var(--gray-900)" />
		<div class="server-select" class:server-select--prod={isProd}>
			{#if isProd}
				<RegionSelect current={currentServer} onSelect={onRegionSelect} />
				<button
					type="button"
					class="advanced-toggle"
					onclick={() => (showAdvanced = !showAdvanced)}
				>
					{showAdvanced ? 'Hide advanced' : 'Advanced, use a custom server'}
				</button>
			{/if}

			{#if !isProd || showAdvanced}
				<Select
					options={servers}
					bind:value={currentServer}
					bind:open={serverSelectOpen}
					onchange={(v) => onServerChange(v as string)}
					placeholder="Select server"
					searchEnabled
					searchPlaceholder="Domain (e.g. api.select-db.com)..."
					createOptionLabel={(q) => `Add server '${q}'`}
					onCreate={onCreateServer}
					canCreate={validateDomain}
					size="sm"
					width={280}
					menuWidth={320}
					isLoading={loading}
				>
					{#snippet optionDisplay(option: SelectOption<string> | null)}
						{#if option}
							<span class="server-option">
								<ServerIndicator
									domain={option.value}
									size={17}
									loaderSize={15}
									showVersion={false}
									showWarning={false}
								/>
								<span class="server-option-label">{option.label}</span>
								{#if option.value !== defaultServer}
									<Button
										label="Remove server"
										leftIcon="minus"
										iconSize={14}
										size="sm"
										emphasis="low"
										noRadius
										noLoader
										onclick={(e) => handleRemoveClick(e, option.value)}
									/>
								{/if}
							</span>
						{:else}
							<span class="server-option-placeholder">Select server</span>
						{/if}
					{/snippet}
				</Select>
			{/if}
			{#if currentServer}
				{#if $serverIndicatorStore[currentServer]?.status === 'loading'}
					<p class="server-version server-version--muted">Checking server…</p>
				{:else if $serverIndicatorStore[currentServer]?.manifest?.backend_version}
					<p class="server-version">
						v.{$serverIndicatorStore[currentServer].manifest?.backend_version}
					</p>
				{/if}
			{/if}

			<div class="divider"></div>
		</div>
	</div>
	<div class="item"></div>

	<div class="item"></div>
	<div class="item center top">
		{#if currentServer && $serverIndicatorStore[currentServer]?.manifest?.warning}
			<div class="alert-slot">
				<Alert
					type={AlertType.Default}
					message={$serverIndicatorStore[currentServer]?.manifest?.warning ?? ''}
					noPulse
				/>
			</div>
		{/if}
		<div class="login-actions">
			<LoginBtn />
		</div>
	</div>
	<div class="item"></div>
</div>

<style>
	.wrapper {
		/* The window is frameless (title bar hidden on macOS), and the login screen
		   renders no tab bar, so nothing here was window-draggable: the whole
		   backdrop is the drag handle instead. */
		--wails-draggable: drag;

		border-top: var(--border);
		border-bottom: var(--border);
		position: relative;
		display: grid;
		grid-template-columns: 1fr 360px 1fr;
		grid-template-rows: 320px 1fr;
		width: 100vw;
		height: 100vh;
		background-color: var(--gray-0);
		overflow: hidden;
	}

	/* --wails-draggable inherits, so anything interactive has to opt back out or
	   its mousedown starts a window drag instead of reaching the control. */
	.server-select,
	.login-actions,
	.alert-slot,
	.wrapper :global(button),
	.wrapper :global(input) {
		--wails-draggable: no-drag;
	}

	.divider {
		width: 100%;
		border-top: var(--border);
		margin-top: var(--space-lg);
	}

	.item {
		padding: 1rem;
		box-sizing: border-box;
	}

	.item:nth-child(3n + 1) {
		border-left: none;
	}

	.item:nth-child(n + 4) {
		border-bottom: var(--border);
	}

	.item.center {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: var(--space-md);
		text-align: center;
	}

	.item.center.top {
		justify-content: start;
		padding-top: var(--space-lg);
	}
	.item.center.bottom {
		gap: var(--space-lg);
		justify-content: end;
		padding-bottom: var(--space-lg);
	}
	.login-actions {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: var(--space-md);
	}

	.server-option {
		display: flex;
		align-items: center;
		gap: var(--space-sm);
		min-width: 0;
		height: 18px;
		width: 100%;
	}

	.server-option-label {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.server-select {
		display: flex;
		flex-direction: column;
		align-items: start;
		gap: var(--space-sm);
	}

	.server-select--prod {
		align-items: center;
	}

	.advanced-toggle {
		display: none;
		background: none;
		border: none;
		padding: 0;
		font-size: var(--fs-xs);
		color: var(--gray-600);
		cursor: pointer;
		align-self: center;
	}

	.advanced-toggle:hover {
		color: var(--gray-900);
	}

	.server-option-placeholder {
		color: var(--gray-800);
	}

	.server-version {
		font-size: var(--fs-xs);
		color: var(--gray-700);
		margin: 0;
	}

	.server-version--muted {
		color: var(--gray-400);
	}
</style>
