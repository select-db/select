<script lang="ts">
	import type { graph } from '$lib/wailsjs/go/models';
	import type { Tab } from '$lib/components/Layout/layoutStore';
	import { updateTab } from '$lib/components/Layout/layoutStore';
	import DatabaseForm, {
		type AvailableDatabases,
		type SavedDatabaseData
	} from './DatabaseForm.svelte';
	import { myPermissions } from '$lib/stores/myPermissionsStore';
	import Alert from '$lib/system/Alert/Alert.svelte';
	import { AlertType } from '$lib/system/Alert/types';

	type Props = {
		tab: Tab;
	};

	let { tab }: Props = $props();

	const database = $derived(tab.database?.node);
	const sshConfig = $derived.by(() => {
		const ssh = database?.ssh;
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

	let currentName = $derived(database?.name ?? '');

	function onSuccess(saved: SavedDatabaseData) {
		if (!database) return;
		updateTab({
			...tab,
			database: {
				node: { ...database, ...saved } as graph.DBInstanceNode
			}
		});
	}
</script>

{#if !database}
	<Alert type={AlertType.Error} message="No database selected" noPulse />
{:else if !$myPermissions.canAccessDb(database.id)}
	<Alert
		type={AlertType.Error}
		message="You don't have permission to access this database."
		noPulse
	/>
{:else}
	{#key database.id}
		<div class="wrapper scrollable">
			<DatabaseForm
				id={database.id}
				uri={database.uri}
				bind:name={currentName}
				db_type={(database.db_type as AvailableDatabases) || 'postgresql'}
				dsn={database.dsn}
				ssh={sshConfig}
				proxified={!!database.proxified}
				folder_id={database.folder_id}
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
</style>
