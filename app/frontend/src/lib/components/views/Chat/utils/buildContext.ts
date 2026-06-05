import { GetOSPathFromURI } from '$lib/wailsjs/go/fs_provider/FSProvider';
import { tryCatch } from '$lib/utils/tryCatch';
import type { ChatContext } from '../tools/context';

async function resolveUri(uri: string): Promise<string> {
	const [abs] = await tryCatch(GetOSPathFromURI, uri);
	return abs ?? uri;
}

export async function buildContext(ctx: ChatContext): Promise<string | null> {
	const lines: string[] = [];

	if (ctx.workspace) {
		const path = await resolveUri(ctx.workspace.uri);
		lines.push(`workspace: ${path}`);
	}

	if (ctx.databases?.length) {
		lines.push('databases:');
		for (const db of ctx.databases) {
			const path = await resolveUri(db.uri);
			lines.push(`  - id: ${db.id}, name: ${db.name} (${db.dialect}), path: ${path}`);
		}
	}

	if (ctx.files?.length) {
		lines.push('files:');
		for (const file of ctx.files) {
			const path = await resolveUri(file.uri);
			lines.push(`  - id: ${file.id}, path: ${path}`);
		}
	}

	if (ctx.currentFile) {
		const cf = ctx.currentFile;
		const path = await resolveUri(cf.uri);
		const cursor =
			cf.cursorPosition != null
				? `line ${cf.cursorPosition.lineNumber}, column ${cf.cursorPosition.column}`
				: '';
		const selection =
			cf.textSelection != null
				? `${cf.textSelection.startLineNumber}:${cf.textSelection.startColumn}-${cf.textSelection.endLineNumber}:${cf.textSelection.endColumn}`
				: '';
		const parts = [`uri: ${cf.uri}`, `id: ${cf.id}`, `path: ${path}`];
		if (cf.folderId) parts.push(`folderId: ${cf.folderId}`);
		if (cursor) parts.push(`cursorPosition: ${cursor}`);
		if (selection) parts.push(`textSelection: ${selection}`);
		if (cf.textSelection?.selectedText != null)
			parts.push(`selectedText: ${JSON.stringify(cf.textSelection.selectedText)}`);
		lines.push(`currentFile: ${parts.join(', ')}`);
	}

	if (lines.length === 0) return null;

	return `<context>\n${lines.join('\n')}\n</context>`;
}
