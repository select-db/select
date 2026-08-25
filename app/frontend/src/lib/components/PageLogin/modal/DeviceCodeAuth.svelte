<script context="module" lang="ts">
	export type DeviceCodeResult = {
		user_code: string;
		device_code: string;
		verification_uri: string;
	};

	export type DeviceCodeAuthProps = {
		/** Modal title, e.g. "Sign in with GitHub" */
		title: string;
		/** Icon name for the header (e.g. "github") */
		icon: import('$lib/system/Icon/types').Icons;
		/** Label for the "open verification URL" button */
		openUrlLabel: string;
		/** Label for the copy code button */
		copyCodeLabel?: string;
		/** Message when getDeviceCode fails */
		initErrorLabel?: string;
		/** Message when auth fails or times out */
		authErrorLabel?: string;
		/** Message when opening URL fails */
		openUrlErrorLabel?: string;
		/** Fetch device code from the provider */
		getDeviceCode: () => Promise<DeviceCodeResult | null>;
		/** Start polling for token; backend will emit "login" on success */
		startPolling: (deviceCode: string) => Promise<void>;
		/** Cancel any ongoing polling */
		cancelPolling: () => void;
	};
</script>

<script lang="ts">
	import { onDestroy, onMount } from 'svelte';

	import { OpenURL } from '$lib/bindings/selectDb/internal/system/system';

	import Alert from '$lib/system/Alert/Alert.svelte';
	import Button from '$lib/system/Button/Button.svelte';
	import { AlertType } from '$lib/system/Alert/types';
	import type { Icons } from '$lib/system/Icon/types';
	import { notify, notifyError } from '$lib/system/Notifications/notificationsStore';
	import ModalHeader from '$lib/system/Modal/ModalHeader.svelte';

	import { tryCatch } from '$lib/utils/tryCatch';

	import ProgressBar from './ProgressBar.svelte';

	export let title: string;
	export let icon: Icons;
	export let openUrlLabel: string;
	export let copyCodeLabel = 'Copy code';
	export let initErrorLabel = 'Failed to initiate login.';
	export let authErrorLabel = 'Authorization failed or timed out';
	export let openUrlErrorLabel = 'Failed to open the login page. Please open the URL manually';
	export let getDeviceCode: () => Promise<DeviceCodeResult | null>;
	export let startPolling: (deviceCode: string) => Promise<void>;
	export let cancelPolling: () => void;

	let userCode: string = '0000-0000';
	let verificationURI: string | null = null;
	let deviceCode: string | null = null;
	let error: string | null = null;

	onMount(async () => {
		const [r, err] = await tryCatch(getDeviceCode);
		if (err || !r) {
			error = initErrorLabel;
			return;
		}
		userCode = r.user_code;
		deviceCode = r.device_code;
		verificationURI = r.verification_uri;

		// Backend emits "login" on success; sessionWall is the only handler (init graph + close modal).
		if (deviceCode) {
			const [, pollErr] = await tryCatch(startPolling, deviceCode);
			if (!pollErr) return;

			error = pollErr?.message ? `${authErrorLabel}: ${pollErr.message}` : authErrorLabel;
			cancelPolling();
		}
	});

	onDestroy(() => {
		cancelPolling();
	});

	async function openVerificationUrl() {
		if (!verificationURI) return;
		const [, err] = await tryCatch(OpenURL, verificationURI);
		if (err) return notifyError(openUrlErrorLabel);
	}

	async function copyCodeToClipboard() {
		const [, err] = await tryCatch(() => navigator.clipboard.writeText(userCode));
		if (err) return notifyError('Failed to copy the code. Please try again manually');
		notify({ type: AlertType.Default, message: 'Code copied to clipboard' });
	}
</script>

<div class="wrapper">
	<ModalHeader {icon} {title} />

	<ProgressBar error={!!error}></ProgressBar>

	<div class="content">
		{#if error}
			<Alert message={error} type={AlertType.Error} noPulse />
		{/if}
		{#if userCode}
			<div class="code">
				<p class="digit">{userCode[0]}</p>
				<p class="digit">{userCode[1]}</p>
				<p class="digit">{userCode[2]}</p>
				<p class="digit">{userCode[3]}</p>
				<p class="separator">{userCode[4]}</p>
				<p class="digit">{userCode[5]}</p>
				<p class="digit">{userCode[6]}</p>
				<p class="digit">{userCode[7]}</p>
				<p class="digit">{userCode[8]}</p>
			</div>
		{/if}
	</div>

	<div class="footer">
		<Button content={copyCodeLabel} emphasis="low" onclick={copyCodeToClipboard}></Button>
		<Button content={openUrlLabel} emphasis="high" onclick={openVerificationUrl}></Button>
	</div>
</div>

<style>
	.wrapper {
		background-color: var(--gray-200);
	}
	.content {
		flex: 1;
		display: flex;
		flex-direction: column;
		gap: var(--space-md);
		padding: var(--space-md);
	}
	.code {
		display: flex;
		justify-content: center;
		align-items: center;
		gap: var(--space-xs);
	}
	.digit,
	.separator {
		font-size: var(--fs-xl);
	}
	.digit {
		border: var(--border-contrast);
		border-radius: var(--br-xs);
		background: var(--gray-300);
		padding: var(--space-xs);
	}
	.footer {
		margin-top: var(--space-sm);
		display: flex;
		justify-content: end;
		padding: var(--space-sm);
		border-top: var(--border);
		gap: var(--space-sm);
	}
</style>
