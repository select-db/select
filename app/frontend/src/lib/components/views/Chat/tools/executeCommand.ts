import { toolDefinition } from '@tanstack/ai';
import { z } from 'zod';
import * as fs from '$lib/wailsjs/go/fs_provider/FSProvider';
import { tryCatch } from '$lib/utils/tryCatch';
import { truncate, toToolError } from './context';

const inputSchema = z.object({
	workspaceId: z
		.string()
		.describe('Workspace ID from context: workspace.id from get_chat_context()'),
	command: z.string().describe('Command to execute (e.g., "grep", "cat", "ls", "find")'),
	args: z
		.array(z.string())
		.describe('Command arguments (e.g., ["-r", "CREATE TABLE", "--include=*.sql", "."] for grep)'),
	workingDir: z
		.string()
		.nullish()
		.transform((val) => val ?? '')
		.default('')
		.describe('Optional: relative path from workspace root. Defaults to workspace root.'),
	timeout: z
		.number()
		.int()
		.positive()
		.nullish()
		.transform((val) => val ?? 0)
		.default(0)
		.describe('Optional: timeout in seconds. 0 means server default (30s).')
});

export const executeCommandDef = toolDefinition({
	name: 'execute_command',
	description: `Executes a sandboxed shell command in the workspace directory. Requires user approval before running.

Use this for:
- Searching across files: grep -r "pattern" --include="*.sql" .
- Partial file reading when read_file is truncated: head -n 50, tail -n 50, grep -A 30 "CREATE TABLE orders"
- Directory exploration: find . -name "*.sql" -type f
- Any file operation not covered by purpose-built tools

Prefer purpose-built tools over this when available:
- Use read_file for reading a known file
- Use get_database_schemas and get_database_table_detail for schema inspection  
- Use execute_query for database reads
- Never use for database operations; use execute_query or execute_statement instead

Commands are sandboxed to the workspace directory. Do not use absolute paths 
outside the workspace or attempt to access system directories.
If exitCode is non-zero, check stderr and diagnose before surfacing to the user.
This tool requires user approval before executing.`,
	needsApproval: true,
	inputSchema,
	outputSchema: z.looseObject({
		stdout: z.string().describe('Command output; primary result to work with'),
		stderr: z
			.string()
			.describe('Error or warning output; non-empty does not always mean failure, check exitCode'),
		exitCode: z.number().describe('0 = success. Non-zero = failure; check stderr for details'),
		error: z
			.string()
			.optional()
			.describe('Present only if command failed to start entirely, before execution')
	})
});

type ImplArgs = z.infer<typeof inputSchema>;

async function executeCommandImpl(args: unknown) {
	const { workspaceId, command, args: cmdArgs, workingDir, timeout } = args as ImplArgs;

	const timeoutMs = (timeout ?? 30) * 1000 + 5000;
	const timeoutPromise = new Promise<never>((_, reject) => {
		setTimeout(() => reject(new Error('Command execution timed out')), timeoutMs);
	});

	const executePromise = fs.ExecuteCommand({
		workspaceId,
		command,
		args: cmdArgs ?? [],
		workingDir: workingDir ?? '',
		timeout: timeout ?? 0
	});

	const [result, err] = await tryCatch(async () => Promise.race([executePromise, timeoutPromise]));

	if (err) {
		return {
			stdout: '',
			stderr: '',
			exitCode: -1,
			error: toToolError(err)
		};
	}

	return {
		stdout: truncate(result.stdout ?? ''),
		stderr: truncate(result.stderr ?? ''),
		exitCode: result.exitCode ?? -1,
		...(result.error ? { error: result.error } : {})
	};
}

export const executeCommandClient = executeCommandDef.client();
export const executeCommandExecutor = executeCommandImpl;
