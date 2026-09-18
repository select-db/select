<script lang="ts">
	import { OpenURL } from '$lib/bindings/selectDb/internal/system/system';

	import Alert from '$lib/system/Alert/Alert.svelte';
	import Button from '$lib/system/Button/Button.svelte';
	import { AlertType } from '$lib/system/Alert/types';
	import { notify, notifyError } from '$lib/system/Notifications/notificationsStore';
	import ModalHeader from '$lib/system/Modal/ModalHeader.svelte';

	import { tryCatch } from '$lib/utils/tryCatch';

	import { cancelLogin, loginFlowStore } from '../loginFlowStore';

	import ProgressBar from './ProgressBar.svelte';

	// Supplied by Modal. Closing leaves the flow running: only Cancel stops it.
	export let onClose: () => void = () => {};

	const PLACEHOLDER_CODE = '0000-0000';

	$: flow = $loginFlowStore;
	$: userCode = flow?.userCode || PLACEHOLDER_CODE;

	function cancel() {
		cancelLogin();
		onClose();
	}

	async function openVerificationUrl() {
		if (!flow?.verificationUri) return;
		const [, err] = await tryCatch(OpenURL, flow.verificationUri);
		if (err) {
			notifyError(
				`Failed to open the ${flow.provider.name} login page. Please open the URL manually`
			);
		}
	}

	async function copyCodeToClipboard() {
		const [, err] = await tryCatch(() => navigator.clipboard.writeText(userCode));
		if (err) return notifyError('Failed to copy the code. Please try again manually');
		notify({ type: AlertType.Default, message: 'Code copied to clipboard' });
	}
</script>

{#if flow}
	<div class="wrapper">
		<ModalHeader icon={flow.provider.icon} title="Sign in with {flow.provider.name}" />

		<ProgressBar error={flow.error !== null} startedAt={flow.startedAt}></ProgressBar>

		<div class="content">
			{#if flow.error}
				<Alert message={flow.error} type={AlertType.Error} noPulse />
			{/if}
			<div class="code" data-test="login.code">
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
		</div>

		<div class="footer">
			<Button content="Cancel" emphasis="low" onclick={cancel}></Button>
			<Button content="Copy code" emphasis="low" onclick={copyCodeToClipboard}></Button>
			<Button content="Go to {flow.provider.name}" emphasis="high" onclick={openVerificationUrl}
			></Button>
		</div>
	</div>
{/if}

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
