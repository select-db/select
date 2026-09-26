<script lang="ts">
	import type * as graph from '$lib/wails/graph';
	import type { Tab } from '$lib/components/Layout/layoutStore';
	import { getTabByNodeId, updateTab } from '$lib/components/Layout/layoutStore';
	import DatasourceForm, {
		type AvailableDatasources,
		type SavedDatasourceData
	} from './DatasourceForm.svelte';
	import { myPermissions } from '$lib/stores/myPermissionsStore';
	import Alert from '$lib/system/Alert/Alert.svelte';
	import { AlertType } from '$lib/system/Alert/types';

	type Props = {
		tab: Tab;
	};

	let { tab }: Props = $props();

	const datasource = $derived(tab.datasource?.node);
	const sshConfig = $derived.by(() => {
		const ssh = datasource?.ssh;
		if (!ssh) return undefined;
		const authMethod: 'password' | 'private_key' | 'agent' | 'key_file' =
			ssh.auth_method === 'private_key' ||
			ssh.auth_method === 'agent' ||
			ssh.auth_method === 'key_file'
				? ssh.auth_method
				: 'password';
		return {
			enabled: ssh.enabled,
			host: ssh.host,
			port: ssh.port,
			user: ssh.user,
			auth_method: authMethod,
			password: ssh.password,
			private_key: ssh.private_key,
			key_path: ssh.key_path ?? '',
			host_key: ssh.host_key ?? ''
		};
	});

	// The form auto-saves on a debounce, so this lands well after the edit — and
	// `tab` is a live prop that by then resolves to whatever tab is active, not
	// the one this form belongs to. Switching tabs mid-save would therefore graft
	// this database onto the tab switched to (a Settings tab, say, would render
	// as a clone of this one). Re-resolve the database's own tab instead.
	function onSuccess(saved: SavedDatasourceData) {
		const savedTab = getTabByNodeId(saved.id);
		if (!savedTab?.datasource) return;
		updateTab({
			...savedTab,
			datasource: {
				...savedTab.datasource,
				node: { ...savedTab.datasource.node, ...saved } as graph.DatasourceNode
			}
		});
	}
</script>

{#if !datasource}
	<Alert type={AlertType.Error} message="No database selected" noPulse />
{:else if !$myPermissions.canAccessDatasource(datasource.id, datasource.proxified)}
	<div class="alert-wrapper">
		<Alert
			type={AlertType.Error}
			message="You don't have permission to access this database."
			noPulse
		/>
	</div>
{:else}
	{#key datasource.id}
		<div class="wrapper scrollable" data-test="datasource.form">
			<DatasourceForm
				id={datasource.id}
				uri={datasource.uri}
				db_type={(datasource.db_type as AvailableDatasources) || 'postgresql'}
				dsn={datasource.dsn}
				ssh={sshConfig}
				proxified={!!datasource.proxified}
				folder_id={datasource.folder_id}
				{onSuccess}
			/>
		</div>
	{/key}
{/if}

<style>
	.wrapper {
		height: 100%;
		overflow-x: hidden;
		overflow-y: auto;
	}

	.alert-wrapper {
		padding: var(--space-sm-md);
		width: fit-content;
	}
</style>
