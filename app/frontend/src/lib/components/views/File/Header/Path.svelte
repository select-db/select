<script lang="ts">
	import { getPathFromUri } from './utils';

	let { uri }: { uri: string } = $props();

	const segments = $derived.by(() => {
		if (!uri) return [];

		// For internal URIs, strip workspace prefix and show the relative path
		if (uri.startsWith('selectdb://')) return getPathFromUri(uri);

		// For plain paths (e.g. git paths), just split on '/'
		return uri.split('/').filter(Boolean);
	});
</script>

<p class="path">{segments.join(' / ')}</p>

<style>
	p {
		font-size: var(--fs-xs);
		color: var(--gray-800);
	}
</style>
