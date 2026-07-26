import { toolDefinition } from '$lib/components/views/Chat/core/chat/tool-definition';
import { z } from 'zod';
import { getAllGroups } from '$lib/components/Layout/layoutStore';
import { ReadFile } from '$lib/wailsjs/go/fs_provider/FSProvider';
import { tryCatch } from '$lib/utils/tryCatch';
import { truncate, toToolError } from '../context';

const inputSchema = z.object({
	uri: z.string().describe('Use files[].uri from get_chat_context(). Never hardcode paths.')
});

const outputSchema = z.union([
	z.object({
		error: z.string().describe('Error message when the read failed'),
		success: z.literal(false)
	}),
	z.object({
		content: z.string().describe('File content. May be truncated for long files.'),
		success: z.literal(true)
	})
]);

export const readFileDef = toolDefinition({
	name: 'read_file',
	description: `Reads the full content of a file from the workspace. Always call this before 
editing a file you haven't read yet; never edit blind.

Use files[].uri from get_chat_context() as the uri parameter. Do not hardcode 
or guess file paths.

If the file contains SQL, validate table and column names against the database 
schema before executing. If content is truncated, use execute_command() with 
head/tail/grep to extract the specific section you need rather than working 
with incomplete content.
If error is returned, surface it to the user and do not proceed.`,
	needsApproval: false,
	inputSchema,
	outputSchema
});

type ImplArgs = z.infer<typeof inputSchema>;

async function readFileImpl(args: unknown): Promise<z.infer<typeof outputSchema>> {
	const { uri } = args as ImplArgs;

	// Temp files: resolve tab and return in-memory content (no backend).
	if (uri.startsWith('temp://')) {
		const tab = getAllGroups()
			.flatMap((g) => g.tabs)
			.find((t) => t.uri === uri);
		const content = tab?.file?.content;
		if (tab == null || content == null) {
			return {
				error: 'Temp file not found or has no content. The tab may have been closed.',
				success: false
			};
		}
		return { content: truncate(content), success: true };
	}

	const [content, err] = await tryCatch(ReadFile, {
		uri
	});

	if (err) {
		return { error: toToolError(err), success: false };
	}

	return {
		content: truncate(content),
		success: true
	};
}

export const readFileClient = readFileDef.client(readFileImpl);
export const readFileExecutor = readFileImpl;
