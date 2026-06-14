<script lang="ts">
	import { Ping, ChooseSSHKeyFile } from '$lib/wailsjs/go/db_client/DbClient';
	import { db_client } from '$lib/wailsjs/go/models';
	import * as fs from '$lib/wailsjs/go/fs_provider/FSProvider';
	import {
		DeleteDatasource,
		GetDatasource,
		UpsertDatasource
	} from '$lib/wailsjs/go/datasource/Datasource';

	import { must, tryCatch } from '$lib/utils/tryCatch';
	import { debounce } from '$lib/utils/debounce';

	import { AlertType } from '$lib/system/Alert/types';
	import Select from '$lib/system/Select/Select.svelte';
	import type { SelectOption } from '$lib/system/Select/Select.types';
	import Input from '$lib/system/Input/Input.svelte';
	import Button from '$lib/system/Button/Button.svelte';
	import SegmentedControl from '$lib/system/SegmentedControl/SegmentedControl.svelte';
	import Icon from '$lib/system/Icon/Icon.svelte';
	import type { Icons } from '$lib/system/Icon/types';
	import { onMount } from 'svelte';
	import { notify, notifyError } from '$lib/system/Notifications/notificationsStore';
	import { modalStore } from '$lib/system/Modal/ModalStore';

	import VariablePicker from '$lib/components/views/File/Header/VariablePicker.svelte';
	import DatabaseFieldHelpModal from './help/DatabaseFieldHelpModal.svelte';
	import { ensureSSHPassphrase } from '$lib/utils/ssh/passphrase';
	import type { DatabaseFieldKey } from './help/fieldHelpContent';
	import type { Component } from 'svelte';
	import Checkbox from '$lib/system/Checkbox/Checkbox.svelte';
	import Loader from '$lib/system/Loader/Loader.svelte';
	import Alert from '$lib/system/Alert/Alert.svelte';

	export type AvailableDatabases = 'sqlite' | 'mysql' | 'postgresql';

	type SSHAuthMethod = 'password' | 'private_key' | 'agent' | 'key_file';

	type SSHConfig = {
		enabled: boolean;
		host: string;
		port: number;
		user: string;
		auth_method: SSHAuthMethod;
		password: string;
		private_key: string;
		key_path: string;
		host_key: string;
	};

	/** Payload passed to onSuccess after a successful save (use to update tab/store). */
	export type SavedDatabaseData = {
		name: string;
		db_type: AvailableDatabases;
		dsn: string;
		ssh: SSHConfig;
		proxified: boolean;
	};

	type DatabaseFormProps = {
		id?: string;
		uri?: string;
		name?: string;
		db_type?: AvailableDatabases;
		dsn?: string;
		ssh?: SSHConfig;
		proxified?: boolean;
		folder_id?: string;

		onSuccess?: (saved: SavedDatabaseData) => void;
	};

	const DIALECT_ICONS: Record<AvailableDatabases, Icons> = {
		sqlite: 'sqlite',
		mysql: 'mysql',
		postgresql: 'postgresql'
	};

	const DSN_PLACEHOLDERS: Record<AvailableDatabases, string> = {
		postgresql:
			'host=prod.db.example.com port=5432 user=read_only password=*** dbname=postgres sslmode=require',
		mysql: 'user:password@tcp(host:3306)/dbname?parseTime=true',
		sqlite: 'file:./local.db or /path/to/database.sqlite'
	};

	let {
		id = $bindable(''),
		uri = $bindable(''),
		name = $bindable(''),
		db_type = $bindable<AvailableDatabases>('postgresql'),
		dsn = $bindable(''),
		ssh = $bindable<SSHConfig>({
			enabled: false,
			host: '',
			port: 22,
			user: '',
			auth_method: 'password',
			password: '',
			private_key: '',
			key_path: '',
			host_key: ''
		}),
		proxified = $bindable(false),
		folder_id = $bindable(''),

		onSuccess
	}: DatabaseFormProps = $props();

	// Local form state (simple primitives)
	// keeps UI reactive and avoids nested mutations on $bindable objects.
	let connectionMode = $state<'dsn' | 'ssh'>(ssh.enabled ? 'ssh' : 'dsn');
	let dsnLocal = $state(dsn);
	let sshHost = $state(ssh.host);
	let sshUser = $state(ssh.user);
	let sshPassword = $state(ssh.password);
	let sshPrivateKey = $state(ssh.private_key);
	let sshKeyPath = $state(ssh.key_path ?? '');
	let sshHostKey = $state(ssh.host_key);
	// Fresh non-proxified tunnels default to the SSH agent: no secrets to enter or store.
	let sshAuthMethod = $state<SSHAuthMethod>(
		!proxified && ssh.auth_method === 'password' && !ssh.password && !ssh.host && !ssh.user
			? 'agent'
			: ssh.auth_method
	);
	let sshPortText = $state(ssh.port ? String(ssh.port) : '22');

	let maxOpenConns = $state(25);
	let maxIdleConns = $state(5);
	let connMaxLifetime = $state(0);
	let connMaxIdleTime = $state(0);

	const isValid = $derived(!!uri && name.trim().length > 0);
	const isNetworked = $derived(db_type !== 'sqlite');

	let remoteLoading = $state(proxified);
	let remoteError = $state<string | null>(null);
	let previousDbType = $state(db_type);

	// When switching to a non-networked dialect, clean up proxy and SSH
	$effect(() => {
		if (db_type === previousDbType) return;
		const wasProxified = proxified;
		previousDbType = db_type;

		if (!isNetworked) {
			connectionMode = 'dsn';
			proxified = false;
			if (wasProxified && id) {
				tryCatch(DeleteDatasource, id);
			}
		}
	});
	let mounted = $state(false);

	onMount(async () => {
		if (!proxified) {
			mounted = true;
			return;
		}
		const [remote, err] = await tryCatch(GetDatasource, id);
		if (err) {
			remoteError = err.message;
			remoteLoading = false;
			mounted = true;
			return;
		}
		if (remote?.dsn) dsnLocal = remote.dsn;
		if (remote?.max_open_conns) maxOpenConns = remote.max_open_conns;
		if (remote?.max_idle_conns) maxIdleConns = remote.max_idle_conns;
		if (remote?.conn_max_lifetime) connMaxLifetime = remote.conn_max_lifetime;
		if (remote?.conn_max_idle_time) connMaxIdleTime = remote.conn_max_idle_time;
		if (remote?.ssh) {
			const [parsed] = tryCatch(() => JSON.parse(remote.ssh) as SSHConfig);
			if (parsed) {
				sshHost = parsed.host ?? '';
				sshUser = parsed.user ?? '';
				sshPassword = parsed.password ?? '';
				sshPrivateKey = parsed.private_key ?? '';
				sshKeyPath = parsed.key_path ?? '';
				sshHostKey = parsed.host_key ?? '';
				sshAuthMethod = parsed.auth_method ?? 'password';
				sshPortText = String(parsed.port || 22);
				connectionMode = parsed.enabled ? 'ssh' : 'dsn';
			}
		}
		remoteLoading = false;
		mounted = true;
	});

	const debouncedSave = debounce(async () => await save(), 600);

	$effect(() => {
		void [
			name,
			db_type,
			dsnLocal,
			connectionMode,
			sshHost,
			sshPortText,
			sshUser,
			sshAuthMethod,
			sshPassword,
			sshPrivateKey,
			sshKeyPath,
			sshHostKey,
			proxified,
			maxOpenConns,
			maxIdleConns,
			connMaxLifetime,
			connMaxIdleTime
		];

		if (!mounted || !isValid) return;

		debouncedSave();
	});

	const RE_VARIABLE = /^\$[A-Za-z_]/;
	const RE_PG_URL = /^postgres(ql)?:\/\/.+@.+\/.+/;
	const RE_PG_KV = /\bhost=\S+/;
	const RE_PG_KV_DB = /\bdbname=\S+/;
	const RE_MYSQL = /^[^@]+@(tcp\()?[^)]+\)?\/.+/;
	const RE_SQLITE = /^(file:|\/)|\.(db|sqlite|sqlite3)$/;
	const RE_PORT = /^\d+$/;
	const RE_HOSTNAME = /^[a-zA-Z0-9._-]+$/;
	const RE_IPV4 = /^\d{1,3}(\.\d{1,3}){3}$/;
	const RE_HOST_KEY = /^\S+\s+(ssh-ed25519|ecdsa-sha2-\S+|ssh-rsa)\s+[A-Za-z0-9+/=]+/;

	const isVariable = (v: string) => RE_VARIABLE.test(v);

	const validateDSN = (v: string) => {
		if (!v) return 'Connection string is required';
		if (isVariable(v)) return null;
		if (db_type === 'postgresql') {
			if (RE_PG_URL.test(v)) return null;
			if (RE_PG_KV.test(v) && RE_PG_KV_DB.test(v)) return null;
			return 'Expected postgres://user:pass@host/db or host=... dbname=...';
		}
		if (db_type === 'mysql') {
			if (RE_MYSQL.test(v)) return null;
			return 'Expected user:pass@tcp(host:3306)/dbname';
		}
		if (db_type === 'sqlite') {
			if (RE_SQLITE.test(v)) return null;
			return 'Expected file:path or /path/to/database.sqlite';
		}
		return null;
	};

	const validatePort = (v: string) => {
		if (!v) return null;
		if (isVariable(v)) return null;
		if (!RE_PORT.test(v)) return 'Must be a number';
		const n = Number(v);
		if (n < 1 || n > 65535) return 'Port must be 1-65535';
		return null;
	};

	const validateHost = (v: string) => {
		if (!v) return 'SSH host is required';
		if (isVariable(v)) return null;
		if (!RE_HOSTNAME.test(v) && !RE_IPV4.test(v)) return 'Expected a hostname or IP address';
		return null;
	};

	const validateUser = (v: string) => {
		if (!v) return 'SSH user is required';
		return null;
	};

	const validatePassword = (v: string) => {
		if (!v && sshAuthMethod === 'password') return 'SSH password is required';
		return null;
	};

	const validatePrivateKey = (v: string) => {
		if (!v && sshAuthMethod === 'private_key') return 'Private key is required';
		return null;
	};

	const validateKeyPath = (v: string) => {
		if (!v && sshAuthMethod === 'key_file') return 'Choose a private key file';
		return null;
	};

	const validateHostKey = (v: string) => {
		if (!v) return proxified ? 'Host key is required for proxified connections' : null;
		if (isVariable(v)) return null;
		if (!RE_HOST_KEY.test(v.trim())) return 'Expected ssh-keyscan output: hostname key-type base64';
		return null;
	};

	const chooseKeyFile = async () => {
		const [path, err] = await tryCatch(ChooseSSHKeyFile);
		if (err) {
			notifyError(err.message);
			return;
		}
		if (!path) return; // user cancelled
		sshKeyPath = path;
		await save();
	};

	const openFieldHelp = (field: DatabaseFieldKey) => {
		modalStore.set({
			content: (() => DatabaseFieldHelpModal) as () => Component,
			props: { field },
			width: 520
		});
	};

	const writeConfigFile = async (data: unknown) => {
		await must(
			tryCatch(fs.Write, {
				uri: `${uri}/db.config.json`,
				content: JSON.stringify(data, null, 2)
			})
		);
	};

	const save = async () => {
		if (!uri) return;

		dsn = dsnLocal;

		ssh.enabled = connectionMode === 'ssh';
		ssh.host = sshHost;
		ssh.user = sshUser;
		ssh.password = sshPassword;
		ssh.private_key = sshPrivateKey;
		ssh.key_path = sshKeyPath;
		ssh.host_key = sshHostKey;
		ssh.auth_method = sshAuthMethod;

		const parsedPort = Number(sshPortText);
		ssh.port = Number.isFinite(parsedPort) && parsedPort > 0 ? parsedPort : 22;

		const savedSsh: SSHConfig = {
			enabled: connectionMode === 'ssh',
			host: sshHost,
			port: ssh.port,
			user: sshUser,
			auth_method: sshAuthMethod,
			password: sshAuthMethod === 'password' ? sshPassword : '',
			private_key: sshAuthMethod === 'private_key' ? sshPrivateKey : '',
			key_path: sshAuthMethod === 'key_file' ? sshKeyPath : '',
			host_key: sshHostKey
		};

		if (proxified) {
			const [, err] = await tryCatch(UpsertDatasource, {
				id,
				db_type,
				name,
				dsn: dsnLocal,
				ssh: JSON.stringify(savedSsh),
				max_open_conns: maxOpenConns,
				max_idle_conns: maxIdleConns,
				conn_max_lifetime: connMaxLifetime,
				conn_max_idle_time: connMaxIdleTime
			});
			if (err) notifyError(err.message);
			await writeConfigFile({
				id,
				name,
				db_type,
				proxified
			});
		} else {
			await writeConfigFile({
				id,
				name,
				db_type,
				dsn: dsnLocal,
				ssh: savedSsh,
				proxified
			});
		}

		onSuccess?.({
			name,
			db_type,
			dsn: dsnLocal,
			ssh: savedSsh,
			proxified
		});
	};

	const attemptPing = async (): Promise<string> => {
		await save();
		return await Ping(
			db_client.PingParams.createFrom({
				DbInstanceID: id,
				db_type,
				dsn: dsnLocal,
				folder_id,
				ssh,
				proxified,
				no_cache: true
			})
		);
	};

	const ping = async () => {
		if (!id) return;

		let error = await attemptPing();

		// Encrypted key file: prompt for the passphrase (stored in memory) and retry.
		if (error && (await ensureSSHPassphrase(sshKeyPath, error, sshHost))) {
			error = await attemptPing();
		}

		if (error) return notifyError(error);

		notify({
			type: AlertType.Success,
			message: 'Database connected'
		});
	};
</script>

<form>
	<div class="group">
		<div class="input-group">
			<div class="standalone-input">
				<p class="label">Type</p>
				<Select
					bind:value={db_type}
					width={140}
					options={[
						{ value: 'sqlite', label: 'SQLite' },
						{ value: 'mysql', label: 'MySQL' },
						{ value: 'postgresql', label: 'PostgreSQL' }
					]}
				>
					{#snippet optionDisplay(option: SelectOption<string> | null)}
						{#if option}
							<span class="dialect-option">
								<Icon icon={DIALECT_ICONS[option.value as AvailableDatabases]} size={16} />
								<span class="dialect-option-label">{option.label}</span>
							</span>
						{:else}
							<span class="dialect-option-label">Select type</span>
						{/if}
					{/snippet}
				</Select>
			</div>

			<div class="standalone-input" style="flex: 1">
				<p class="label">Name</p>
				<Input bind:value={name} placeholder="Prod read-only (RDS)" />
			</div>
		</div>
	</div>

	<div class="group">
		{#if isNetworked}
			<div class="standalone-input">
				<p class="label with-help">
					<span>Proxy connection</span>
					<button type="button" class="help" onclick={() => openFieldHelp('proxy_connection')}>
						<Icon icon="info" size={12} />
					</button>
				</p>
				<Checkbox
					bind:checked={proxified}
					onchange={async (checked) => {
						if (checked) {
							// cleanup local config file
							await writeConfigFile({
								id,
								name,
								db_type,
								proxified: checked
							});
						} else {
							await must(tryCatch(DeleteDatasource, id));
						}
					}}
					label="Proxified"
					size="sm"
				/>
			</div>

			<div class="standalone-input">
				<p class="label with-help">
					<span>Connection</span>
					<button type="button" class="help" onclick={() => openFieldHelp('connection_mode')}>
						<Icon icon="info" size={12} />
					</button>
				</p>
				<SegmentedControl
					options={[
						{
							id: 'dsn',
							label: 'DSN only'
						},
						{
							id: 'ssh',
							label: 'DSN + SSH tunnel'
						}
					]}
					value={connectionMode}
					onSelect={(mode) => (connectionMode = mode as 'dsn' | 'ssh')}
				/>
			</div>
		{/if}

		{#if proxified && remoteLoading}
			<div class="remote-state">
				<Loader size={18} />
				<p>Loading credentials…</p>
			</div>
		{:else if proxified && remoteError}
			<Alert type={AlertType.Error} message={remoteError} noPulse />
		{:else}
			<div class="proxifiable-group" class:proxified>
				<div class="standalone-input">
					<p class="label with-help">
						<span>Database connection string (DSN)</span>
						<button type="button" class="help" onclick={() => openFieldHelp('dsn')}>
							<Icon icon="info" size={12} />
						</button>
					</p>
					<div class="action-wrapper">
						<Input
							bind:value={dsnLocal}
							type="text"
							placeholder={DSN_PLACEHOLDERS[db_type]}
							style="flex-grow: 1;"
							validator={validateDSN}
						/>
						{#if uri && !proxified}
							<VariablePicker
								{uri}
								iconSize={14}
								style="height: 32px"
								onchange={(v) => (dsnLocal = v)}
							/>
						{/if}
					</div>
				</div>

				{#if connectionMode === 'ssh'}
					<div class="group ssh">
						<div class="input-group">
							<div class="standalone-input" style="flex: 1">
								<p class="label with-help">
									<span>SSH user</span>
									<button type="button" class="help" onclick={() => openFieldHelp('ssh_user')}>
										<Icon icon="info" size={12} />
									</button>
								</p>
								<div class="action-wrapper">
									<Input
										bind:value={sshUser}
										placeholder={proxified ? 'ubuntu' : '$DB_SSH_USER'}
										style="flex-grow: 1;"
										validator={validateUser}
									/>
									{#if uri && !proxified}
										<VariablePicker
											{uri}
											iconSize={14}
											style="height: 32px"
											onchange={(v) => (sshUser = v)}
										/>
									{/if}
								</div>
							</div>
							<div class="standalone-input" style="flex: 1">
								<p class="label with-help">
									<span>SSH tunnel host</span>
									<button type="button" class="help" onclick={() => openFieldHelp('ssh_host')}>
										<Icon icon="info" size={12} />
									</button>
								</p>
								<div class="action-wrapper">
									<Input
										bind:value={sshHost}
										placeholder="ec2-203-0-113-42.eu-west-1.compute.amazonaws.com"
										style="flex-grow: 1;"
										validator={validateHost}
									/>
									{#if uri && !proxified}
										<VariablePicker
											{uri}
											iconSize={14}
											style="height: 32px"
											onchange={(v) => (sshHost = v)}
										/>
									{/if}
								</div>
							</div>
							<div class="standalone-input">
								<p class="label with-help">
									<span>SSH port</span>
									<button type="button" class="help" onclick={() => openFieldHelp('ssh_port')}>
										<Icon icon="info" size={12} />
									</button>
								</p>
								<Input
									bind:value={sshPortText}
									placeholder="22"
									validator={validatePort}
									style="width: 45px;"
								/>
							</div>
						</div>
						{#if proxified}
							<div class="input-group">
								<div class="standalone-input" style="flex: 1;">
									<p class="label with-help">
										<span>SSH host key (required)</span>
										<button
											type="button"
											class="help"
											onclick={() => openFieldHelp('ssh_host_key')}
										>
											<Icon icon="info" size={12} />
										</button>
									</p>
									<div class="action-wrapper">
										<Input
											bind:value={sshHostKey}
											placeholder="ssh-ed25519 AAAA…"
											style="flex-grow: 1;"
											validator={validateHostKey}
										/>
									</div>
								</div>
							</div>
						{/if}

						<div class="input-group">
							<div class="standalone-input">
								<p class="label with-help">
									<span>Authentication method</span>
									<button
										type="button"
										class="help"
										onclick={() => openFieldHelp('ssh_auth_method')}
									>
										<Icon icon="info" size={12} />
									</button>
								</p>
								<Select
									bind:value={sshAuthMethod}
									width={180}
									options={proxified
										? [
												{ value: 'password', label: 'Password' },
												{ value: 'private_key', label: 'Private key' }
											]
										: [
												{ value: 'agent', label: 'SSH agent' },
												{ value: 'key_file', label: 'Key file' },
												{ value: 'password', label: 'Password' },
												{ value: 'private_key', label: 'Private key' }
											]}
								/>
							</div>
							{#if sshAuthMethod === 'password'}
								<div class="standalone-input" style="flex: 1;">
									<p class="label with-help">
										<span>SSH password</span>
										<button
											type="button"
											class="help"
											onclick={() => openFieldHelp('ssh_password')}
										>
											<Icon icon="info" size={12} />
										</button>
									</p>
									<div class="action-wrapper">
										<Input
											bind:value={sshPassword}
											type="text"
											placeholder={proxified ? 'SSH password' : '$PROD_DB_SSH_PASSWORD'}
											style="flex-grow: 1;"
											validator={validatePassword}
										/>
										{#if uri && !proxified}
											<VariablePicker
												{uri}
												iconSize={14}
												style="height: 32px"
												onchange={(v) => (sshPassword = v)}
											/>
										{/if}
									</div>
								</div>
							{/if}
						</div>

						{#if sshAuthMethod === 'private_key'}
							<div class="standalone-input">
								<p class="label with-help">
									<span>Private key (raw)</span>
									<button
										type="button"
										class="help"
										onclick={() => openFieldHelp('ssh_private_key')}
									>
										<Icon icon="info" size={12} />
									</button>
								</p>
								<div class="action-wrapper">
									<Input
										bind:value={sshPrivateKey}
										multiline={proxified}
										type="text"
										rows={8}
										placeholder={proxified
											? 'Paste full private key'
											: '$PROD_DB_SSH_KEY (full private key)'}
										style="flex-grow: 1;"
										validator={validatePrivateKey}
									/>
									{#if uri && !proxified}
										<VariablePicker
											{uri}
											iconSize={14}
											style="height: 32px"
											onchange={(v) => (sshPrivateKey = v)}
										/>
									{/if}
								</div>
							</div>
						{/if}
						{#if !proxified && sshAuthMethod === 'key_file'}
							<div class="standalone-input" style="flex: 1;">
								<p class="label with-help">
									<span>Private key file</span>
									<button
										type="button"
										class="help"
										onclick={() => openFieldHelp('ssh_private_key')}
									>
										<Icon icon="info" size={12} />
									</button>
								</p>
								<div class="action-wrapper">
									<div class="host-key-readonly" class:empty={!sshKeyPath}>
										{sshKeyPath || 'No key file chosen'}
									</div>
									<Button content="Choose key file" emphasis="low" onclick={chooseKeyFile} />
								</div>
								{#if validateKeyPath(sshKeyPath)}
									<p class="host-key-msg error">{validateKeyPath(sshKeyPath)}</p>
								{:else}
									<p class="host-key-msg">
										If the key is encrypted, you'll be asked for the passphrase when you connect.
									</p>
								{/if}
							</div>
						{/if}
					</div>
				{/if}

				{#if proxified}
					<div class="divider pool-divider"></div>
					<div class="input-group">
						<div class="standalone-input" style="flex: 1">
							<p class="label">Max open connections</p>
							<Input type="number" min={0} bind:value={maxOpenConns} placeholder="25" />
						</div>
						<div class="standalone-input" style="flex: 1">
							<p class="label">Max idle connections</p>
							<Input type="number" min={0} bind:value={maxIdleConns} placeholder="5" />
						</div>
					</div>
					<div class="input-group">
						<div class="standalone-input" style="flex: 1">
							<p class="label">Max lifetime (s, 0 = no limit)</p>
							<Input type="number" min={0} bind:value={connMaxLifetime} placeholder="0" />
						</div>
						<div class="standalone-input" style="flex: 1">
							<p class="label">Max idle time (s, 0 = no limit)</p>
							<Input type="number" min={0} bind:value={connMaxIdleTime} placeholder="0" />
						</div>
					</div>
				{/if}
			</div>
		{/if}
	</div>

	<div class="group">
		<div class="standalone-input">
			<div class="action-wrapper">
				<Button content="Test connection" emphasis="low" onclick={ping} />
			</div>
		</div>
	</div>
</form>

<style>
	form {
		display: flex;
		flex-direction: column;
		align-items: stretch;
		gap: var(--space-lg);
		padding: var(--space-md) var(--space-sm-md);
	}
	form .group {
		display: flex;
		flex-direction: column;
		gap: var(--space-lg);
		max-width: 600px;
		min-width: 360px;
	}

	form .group.ssh {
		background-color: var(--gray-100);
		padding: none;
		max-width: none;
		border: var(--border);
		border-radius: var(--br-xs);
		padding: var(--space-md) var(--space-sm-md);
	}
	form .input-group {
		display: flex;
		justify-content: start;
		gap: var(--space-md);
	}
	form .standalone-input {
		position: relative;
		display: flex;
		flex-direction: column;
		gap: var(--space-xs);
	}
	form .label {
		padding-left: var(--space-xxs);
		margin-bottom: var(--space-xs);
	}
	form .label,
	form .label span {
		color: var(--gray-800);
	}

	.action-wrapper {
		display: flex;
		align-items: stretch;
		gap: var(--space-xs);
	}
	.proxifiable-group {
		display: flex;
		flex-direction: column;
		gap: var(--space-md);
	}
	.proxified {
		background-color: var(--gray-100);
		padding: var(--space-sm-md);
		border-radius: var(--br-md);
		border: var(--border);
		border-color: var(--blue);
	}
	.proxified .group.ssh {
		border-color: var(--gray-100);
		padding: 0;
	}
	.proxified .divider {
		border-color: var(--gray-100);
	}

	.dialect-option {
		display: inline-flex;
		align-items: center;
		gap: var(--space-sm);
	}

	.dialect-option-label {
		white-space: nowrap;
	}

	.label.with-help {
		display: inline-flex;
		align-items: center;
		gap: var(--space-xs);
	}

	button.help {
		border: none;
		background: transparent;
		padding: 0;
		margin: 0;
		display: inline-flex;
		align-items: center;
		color: var(--gray-700);
	}

	:global(button.help:hover svg) {
		stroke: var(--gray-1000);
	}

	.host-key-readonly {
		flex-grow: 1;
		min-width: 0;
		display: flex;
		align-items: center;
		height: 32px;
		padding: 0 var(--space-sm);
		border: var(--border);
		border-radius: var(--br-xs);
		background-color: var(--gray-0);
		color: var(--gray-900);
		font-size: var(--fs-sm);
		overflow: hidden;
		white-space: nowrap;
		text-overflow: ellipsis;
	}
	.host-key-readonly.empty {
		color: var(--gray-800);
	}

	.host-key-msg {
		margin-top: var(--space-xs);
		font-size: 12px;
		color: var(--gray-700);
	}
	.host-key-msg.error {
		color: var(--red);
	}
</style>
