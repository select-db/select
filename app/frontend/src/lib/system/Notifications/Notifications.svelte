<script lang="ts">
	import { fly } from 'svelte/transition';
	import { linear } from 'svelte/easing';
	import {
		notificationStore,
		pauseNotification,
		resumeNotification,
		dismissNotification
	} from '$lib/system/Notifications/notificationsStore';
	import { onDestroy } from 'svelte';
	import Alert from '../Alert/Alert.svelte';

	let notifications = $notificationStore;

	const unsubscribe = notificationStore.subscribe((val) => {
		notifications = val;
	});

	onDestroy(unsubscribe);

	function flyInOut(node: Element) {
		return fly(node, {
			x: 500,
			duration: 150,
			easing: linear
		});
	}
</script>

<div class="wrapper" class:show={notifications.length > 0}>
	{#each notifications.slice().reverse() as { id, type, message, copyable } (id)}
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div
			class="alert-wrapper"
			in:flyInOut
			out:flyInOut
			onmouseenter={() => pauseNotification(id)}
			onmouseleave={() => resumeNotification(id)}
		>
			<Alert {type} {message} {copyable} onClose={() => dismissNotification(id)} noPulse />
		</div>
	{/each}
</div>

<style>
	.wrapper {
		position: fixed;
		bottom: 0;
		right: 0;
		display: flex;
		flex-direction: column;
		align-items: end;
		gap: var(--space-sm);
		z-index: 6;
	}

	.wrapper.show {
		padding: var(--space-md);
	}

	.alert-wrapper {
		will-change: transform, opacity;
	}

	:global(.alert-wrapper:hover .alert) {
		background-color: var(--gray-100);
	}
</style>
