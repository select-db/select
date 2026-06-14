export type DatabaseFieldKey =
	| 'connection_mode'
	| 'proxy_connection'
	| 'dsn'
	| 'ssh_host'
	| 'ssh_port'
	| 'ssh_user'
	| 'ssh_auth_method'
	| 'ssh_password'
	| 'ssh_private_key'
	| 'ssh_host_key';

export type HelpTable = {
	headers: [string, string];
	rows: [string, string][];
};

export type DatabaseFieldHelp = {
	title: string;
	what: string;
	where: string;
	command?: string;
	table?: HelpTable;
};

export const databaseFieldHelpContent: Record<DatabaseFieldKey, DatabaseFieldHelp> = {
	proxy_connection: {
		title: 'Proxy connection',
		what: 'Credentials are stored encrypted on the server and never sent to your local machine. All queries route through a shared server-side connection pool.',
		where:
			'Enable for team databases where credentials should stay hidden from end users. $variables are not supported in proxified mode.',
		table: {
			headers: ['Mode', 'Credentials stored'],
			rows: [
				['Local', 'In db.config.json on your machine.'],
				['Proxified', 'Encrypted on the remote server. Local config holds no DSN or SSH data.']
			]
		}
	},
	connection_mode: {
		title: 'Connection mode',
		what: 'Whether you connect directly with the DSN, or first open an SSH tunnel then use the same DSN through it.',
		where:
			'Use "DSN only" if your app can reach the database directly. Use "DSN + SSH tunnel" when you must SSH into a jump host (bastion) before reaching the database.',
		table: {
			headers: ['Mode', 'Use when'],
			rows: [
				['DSN only', 'Your app can reach the DB directly (same VPN, local, or public endpoint).'],
				['DSN + SSH tunnel', 'DB is only reachable via a jump/bastion host (e.g. EC2).']
			]
		}
	},
	dsn: {
		title: 'Database connection string (DSN)',
		what: 'The full connection string the driver uses to reach your database: host, port, user, password, database name, and SSL options.',
		where:
			'Copy it from your application config or cloud console, or build it manually. For PostgreSQL with SSH, prefer the key=value format so we can read host and port.'
	},
	ssh_host: {
		title: 'SSH tunnel host',
		what: 'The server you SSH into to reach the database. This is your jump / bastion host, not the database host itself.',
		where:
			'Use the hostname or IP of the machine that can see the database from inside the network (for example an EC2 instance).'
	},
	ssh_port: {
		title: 'SSH port',
		what: 'The TCP port used for the SSH connection to the tunnel host.',
		where: 'Typically 22. Only change it if your SSH server listens on a non-standard port.'
	},
	ssh_user: {
		title: 'SSH user',
		what: 'The Linux / system user you connect as on the SSH tunnel host.',
		where: 'From your SSH configuration or cloud console for the bastion host.'
	},
	ssh_auth_method: {
		title: 'SSH authentication method',
		what: 'How we authenticate to the SSH tunnel host. In local mode prefer the SSH agent or a key file so no secret is stored on disk.',
		where: 'Match how you normally SSH into this host from your terminal.',
		table: {
			headers: ['Method', 'When to use'],
			rows: [
				[
					'SSH agent',
					'Recommended (local). Uses your running agent (SSH_AUTH_SOCK). Nothing stored; encrypted keys work transparently.'
				],
				[
					'Key file',
					'Local. Pick a key file on disk; only the path is saved, contents are read at connect time. Prompts for a passphrase if encrypted.'
				],
				[
					'Password',
					'You type a password when connecting (e.g. ssh user@host). Prefer an .env $variable over a raw value.'
				],
				['Private key', 'Paste the raw key (proxified) or reference an .env $variable (local).']
			]
		}
	},
	ssh_password: {
		title: 'SSH password',
		what: 'The SSH account password for the SSH user, if you selected password authentication.',
		where: 'Use an environment variable instead of typing the raw password when possible.'
	},
	ssh_private_key: {
		title: 'SSH private key',
		what: 'The key used to log into the tunnel host.',
		where:
			'Local: choose a key file (e.g. ~/.ssh/id_ed25519) or reference an .env variable. Proxified: paste the full PEM key.'
	},
	ssh_host_key: {
		title: 'SSH host key',
		what: 'The public key of your SSH tunnel host. SELECT checks it on every connection to make sure you are reaching the right server.',
		where: 'Paste ssh-keyscan output (required); the server has no access to your machine.',
		command: 'ssh-keyscan your-bastion-host',
		table: {
			headers: ['Example output', 'Key type'],
			rows: [
				['bastion.example.com ssh-ed25519 AAAAC3Nza...', 'Ed25519'],
				['bastion.example.com ecdsa-sha2-nistp256 AAAAE2Vjd...', 'ECDSA'],
				['bastion.example.com ssh-rsa AAAAB3NzaC1yc2...', 'RSA']
			]
		}
	}
};
