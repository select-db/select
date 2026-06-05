<script lang="ts">
	import { onMount } from 'svelte';
	import { EventsOn } from '$lib/wailsjs/runtime';
	import Icon from '$lib/system/Icon/Icon.svelte';
	import type { Icons } from '$lib/system/Icon/types';

	type Quality = 'excellent' | 'good' | 'average' | 'poor' | 'offline' | 'unknown';

	let network: Quality = 'unknown';

	const quality: {
		[key in Quality]: {
			icon: Icons;
			color: string;
		};
	} = {
		excellent: {
			icon: 'wifi-excellent',
			color: 'var(--green)'
		},
		good: {
			icon: 'wifi-good',
			color: 'var(--green-dark)'
		},
		average: {
			icon: 'wifi-average',
			color: 'var(--yellow)'
		},
		poor: {
			icon: 'wifi-poor',
			color: 'var(--orange)'
		},
		offline: {
			icon: 'wifi-offline',
			color: 'var(--red)'
		},
		unknown: {
			icon: 'wifi-offline',
			color: 'var(--red)'
		}
	};

	onMount(() => {
		EventsOn('networkQuality', (data) => {
			network = data.quality as Quality;
		});
	});
</script>

<div class="wrapper">
	<Icon icon={quality[network].icon} stroke={quality[network].color} size={22} />
</div>

<style>
	.wrapper {
		display: flex;
		align-items: center;
	}
</style>
