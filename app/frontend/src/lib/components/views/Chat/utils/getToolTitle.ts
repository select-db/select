import { getAllGroups } from '$lib/components/Layout/layoutStore';

function parseArgs(args: string | undefined): Record<string, unknown> {
	if (args == null || typeof args !== 'string') return {};
	const t = args.trim();
	if (!t) return {};
	try {
		const parsed = JSON.parse(t);
		return typeof parsed === 'object' && parsed !== null ? parsed : {};
	} catch {
		return {};
	}
}

function basename(uri: string): string {
	const segment = uri.split(/[/\\]/).filter(Boolean).pop();
	if (!segment) return uri;
	try {
		return decodeURIComponent(segment);
	} catch {
		return segment;
	}
}

/** Temp URIs end in a UUID; show the editor tab title instead. */
function fileLabel(uri: string): string {
	if (uri.startsWith('temp://')) {
		for (const g of getAllGroups()) {
			const t = g.tabs.find((x) => x.uri === uri);
			if (t?.file?.node?.name) return t.file.node.name;
		}
	}
	return basename(uri);
}

function truncateLabel(s: string, max = 40): string {
	if (s.length <= max) return s;
	return s.slice(0, max).trim() + '…';
}

export type ToolTitleParts = {
	prefix: string;
	code?: string;
	suffix?: string;
};

export function getToolTitleParts(name: string, argumentsJson?: string): ToolTitleParts {
	const args = parseArgs(argumentsJson);
	const a = args as Record<string, string | undefined>;

	switch (name) {
		case 'read_file': {
			const uri = a.uri;
			if (uri) return { prefix: 'Read', code: truncateLabel(fileLabel(uri)) };
			return { prefix: 'Read file' };
		}
		case 'edit_file': {
			const uri = a.uri;
			if (uri) return { prefix: 'Edit', code: truncateLabel(fileLabel(uri)) };
			return { prefix: 'Edit file' };
		}
		case 'get_database_schemas': {
			return { prefix: 'Read database schemas' };
		}
		case 'get_database_table_detail': {
			const tableName = a.tableName;
			if (tableName)
				return { prefix: 'Read', code: truncateLabel(tableName), suffix: 'table' };
			return { prefix: 'Read table detail' };
		}
		case 'execute_query':
			return { prefix: 'Execute query' };
		case 'execute_statement':
			return { prefix: 'Execute statement' };
		case 'plan_query':
			return { prefix: 'Plan query' };
		case 'explain_query':
			return { prefix: 'Explain query' };
		case 'execute_command': {
			const command = a.command;
			if (command) return { prefix: 'Run', code: truncateLabel(command) };
			return { prefix: 'Run command' };
		}
		default: {
			return {
				prefix: name
					.split('_')
					.map((w) => w.charAt(0).toUpperCase() + w.slice(1).toLowerCase())
					.join(' ')
			};
		}
	}
}
