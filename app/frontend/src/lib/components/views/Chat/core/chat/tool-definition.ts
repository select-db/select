import { z } from 'zod';
import type { AnyClientTool, ClientTool } from './types';

export type { AnyClientTool, ClientTool };

export interface ToolDefinitionConfig<TInput = unknown, TOutput = unknown> {
	name: string;
	description: string;
	inputSchema?: TInput;
	outputSchema?: TOutput;
	needsApproval?: boolean;
	metadata?: Record<string, unknown>;
}

export interface ToolDefinition<TInput = unknown, TOutput = unknown>
	extends ToolDefinitionConfig<TInput, TOutput> {
	__toolSide: 'definition';
	client: (execute?: (args: never) => unknown) => ClientTool;
}

/**
 * Define a tool once, then materialize a client tool with `.client(execute?)`.
 * A client tool with an `execute` fn is run automatically by the chat client;
 * without one, the app supplies the result via `addToolResult`.
 */
export function toolDefinition<TInput = unknown, TOutput = unknown>(
	config: ToolDefinitionConfig<TInput, TOutput>
): ToolDefinition<TInput, TOutput> {
	return {
		__toolSide: 'definition',
		...config,
		client(execute?: (args: never) => unknown): ClientTool {
			return {
				__toolSide: 'client',
				name: config.name,
				description: config.description,
				inputSchema: config.inputSchema,
				outputSchema: config.outputSchema,
				needsApproval: config.needsApproval,
				metadata: config.metadata,
				execute: execute as ClientTool['execute']
			};
		}
	};
}

/** Collect client tools into a tuple (identity — preserves literal names for typing). */
export function clientTools<const T extends AnyClientTool[]>(...tools: T): T {
	return tools;
}

/**
 * Convert a tool's input schema to a JSON Schema object for the provider.
 * Zod schemas are converted via zod's built-in `toJSONSchema`; anything that is
 * already a plain JSON Schema object is passed through. `$schema` is stripped.
 */
export function toParametersJsonSchema(schema: unknown): Record<string, unknown> {
	if (!schema || typeof schema !== 'object') {
		return { type: 'object', properties: {} };
	}

	let json: Record<string, unknown>;
	if (schema instanceof z.ZodType) {
		json = z.toJSONSchema(schema, { target: 'draft-7', io: 'input' }) as Record<string, unknown>;
	} else {
		json = { ...(schema as Record<string, unknown>) };
	}

	delete json.$schema;
	if (!json.type && json.properties) json.type = 'object';
	return json;
}
