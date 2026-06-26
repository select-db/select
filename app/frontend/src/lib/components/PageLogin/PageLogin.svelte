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
	} from '$lib/wailsjs/go/server/Server';
	import type { Component } from 'svelte';
	import ConfirmRemoveServerModal from './ConfirmRemoveServerModal.svelte';
	import { GetAppEnv } from '$lib/wailsjs/go/system/System';
	import RegionSelect from './RegionSelect.svelte';

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
		<svg class="logo" viewBox="0 -880 4487 1024" fill="currentColor"
			><g transform="scale(1, -1)"
				><path
					transform="translate(0, 0)"
					d="M710 525Q710 497 690 478Q671 458 643 458H243Q224 457 211.5 449.5Q199 442 193.0 432.5Q187 423 187 413Q187 403 193.0 393.5Q199 384 211.5 376.5Q224 369 243 368H527Q597 366 642.5 335.5Q688 305 707.5 265.5Q727 226 727 185Q727 145 707.0 105.5Q687 66 642.0 35.5Q597 5 527 5H521Q515 4 509 4H131Q103 4 83.5 23.5Q64 43 64 71Q64 99 84 118Q103 138 131 138H533Q552 137 564.5 145.0Q577 153 583.0 163.5Q589 174 589 185Q589 192 583.0 204.5Q577 217 564.5 225.0Q552 233 533 234H249Q179 236 133.5 266.0Q88 296 68.5 334.5Q49 373 49 413Q49 452 69.0 490.5Q89 529 134.0 559.0Q179 589 249 591H255Q261 592 267 592H643Q671 592 690.5 572.5Q710 553 710 525Z"
				/><path
					transform="translate(776, 0)"
					d="M249 592H295H635Q663 591 681.5 571.0Q700 551 700 523Q695 463 635 458H283Q245 456 220.0 429.0Q195 402 183.5 367.5Q172 333 172 297Q172 261 183.5 226.5Q195 192 220.0 165.0Q245 138 283 136H635Q663 135 681.5 115.0Q700 95 700 67Q699 40 680.5 21.5Q662 3 635 2H268H249Q179 6 133.5 55.5Q88 105 68.5 168.0Q49 231 49 297Q49 363 69.0 426.0Q89 489 134.0 538.5Q179 588 249 592ZM698 301Q698 273 678 254Q659 234 631 234H329Q301 234 281.5 253.5Q262 273 262 301Q262 329 281 349Q301 368 329 368H631Q659 368 678.5 348.5Q698 329 698 301Z"
				/><path
					transform="translate(1513, 0)"
					d="M130 608Q158 608 178.0 589.5Q198 571 199 543V302Q198 261 204.5 226.5Q211 192 236.0 165.0Q261 138 299 136H619Q647 135 665.5 115.0Q684 95 684 67Q683 40 664.5 21.5Q646 3 619 2H265Q195 6 149.5 49.5Q104 93 84 156Q65 219 64 290V543Q65 570 84.0 588.5Q103 607 130 608Z"
				/><path
					transform="translate(2229, 0)"
					d="M249 592H295H635Q663 591 681.5 571.0Q700 551 700 523Q695 463 635 458H283Q245 456 220.0 429.0Q195 402 183.5 367.5Q172 333 172 297Q172 261 183.5 226.5Q195 192 220.0 165.0Q245 138 283 136H635Q663 135 681.5 115.0Q700 95 700 67Q699 40 680.5 21.5Q662 3 635 2H268H249Q179 6 133.5 55.5Q88 105 68.5 168.0Q49 231 49 297Q49 363 69.0 426.0Q89 489 134.0 538.5Q179 588 249 592ZM698 301Q698 273 678 254Q659 234 631 234H329Q301 234 281.5 253.5Q262 273 262 301Q262 329 281 349Q301 368 329 368H631Q659 368 678.5 348.5Q698 329 698 301Z"
				/><path
					transform="translate(2966, 0)"
					d="M291 458Q253 456 227.5 429.0Q202 402 191.0 367.5Q180 333 180 297Q180 261 190.5 226.0Q201 191 227.0 164.5Q253 138 291 136H621Q684 130 690 67Q689 39 669.0 20.5Q649 2 621 2H302H256Q186 6 141.0 55.5Q96 105 76.5 168.0Q57 231 57 297Q57 346 68.0 394.0Q79 442 102.5 485.5Q126 529 165.5 560.0Q205 591 256 592H275H621Q684 586 690 523Q690 495 669.5 476.5Q649 458 621 458Z"
				/><path
					transform="translate(3685, 0)"
					d="M44 525Q44 553 63.5 572.5Q83 592 111 592H693Q721 592 741 573Q760 553 760 525Q760 497 740.5 477.5Q721 458 693 458H470V53Q464 -7 404 -12Q376 -12 355.5 6.5Q335 25 335 53V458H111Q83 458 64 478Q44 497 44 525Z"
				/></g
			></svg
		>
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
			<Alert
				type={AlertType.Default}
				message={$serverIndicatorStore[currentServer]?.manifest?.warning ?? ''}
				noPulse
			/>
		{/if}
		<div class="login-actions">
			<LoginBtn />
		</div>
	</div>
	<div class="item"></div>
</div>

<style>
	.logo {
		height: 1.5rem;
		color: var(--gray-900);
	}

	.wrapper {
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
