<script lang="ts">
	import DeviceCodeAuth from './DeviceCodeAuth.svelte';
	import {
		GetAccessToken,
		GetDeviceCode,
		CancelAccessTokenPolling
	} from '$lib/wailsjs/go/auth/GithubAuth';
</script>

<DeviceCodeAuth
	title="Sign in with GitHub"
	icon="github"
	openUrlLabel="Go to GitHub"
	copyCodeLabel="Copy code"
	initErrorLabel="Failed to initiate GitHub login."
	authErrorLabel="Authorization failed or timed out"
	openUrlErrorLabel="Failed to open the GitHub login page. Please open the URL manually"
	getDeviceCode={GetDeviceCode}
	startPolling={GetAccessToken}
	cancelPolling={CancelAccessTokenPolling}
/>
