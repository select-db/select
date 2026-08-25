/**
 * The workspace graph as the app sees it.
 *
 * The bindings render Go pointer slices (`[]*FileNode`) as `(FileNode | null)[]`
 * because JSON could carry a null in them; the graph the backend builds never
 * does. Rather than guard at every read, the nulls are dropped once here — at
 * the calls and events that bring graph data into the app — and the types
 * exported below say so.
 *
 * Anything new that returns graph data belongs in this module, so the
 * invariant keeps holding at a single boundary.
 */
import * as graphService from '$lib/bindings/selectDb/internal/graph/graph';
import * as models from '$lib/bindings/selectDb/internal/graph/models';
import type * as coreModels from '$lib/bindings/github.com/selectDb/dialect/core/models';
import * as sqlLangService from '$lib/bindings/selectDb/internal/sqllang/sqllang';
import type * as sqlLangModels from '$lib/bindings/selectDb/internal/sqllang/models';
import * as dbClientService from '$lib/bindings/selectDb/internal/db_client/dbclient';
import type * as dbClientModels from '$lib/bindings/selectDb/internal/db_client/models';
import * as searchService from '$lib/bindings/selectDb/internal/search/search';
import type * as searchModels from '$lib/bindings/selectDb/internal/search/models';

/**
 * Removes `null` from array element types. Nullable values that are not array
 * entries — an absent `ssh` config, an unknown `errorPosition` — are left
 * alone, because those nulls are real.
 */
export type NonNullItems<T> = unknown extends T
	? T
	: T extends (infer U)[]
		? NonNullItems<NonNullable<U>>[]
		: T extends object
			? { [K in keyof T]: NonNullItems<T[K]> }
			: T;

export type WorkspaceNode = NonNullItems<models.WorkspaceNode>;
export type FolderNode = NonNullItems<models.FolderNode>;
export type FileNode = NonNullItems<models.FileNode>;
export type DBInstanceNode = NonNullItems<models.DBInstanceNode>;
export type DBInstanceItemNode = NonNullItems<models.DBInstanceItemNode>;
export type QueryResult = NonNullItems<models.QueryResult>;
export type ExplainResult = NonNullItems<models.ExplainResult>;
export type ColumnMetadata = NonNullItems<models.ColumnMetadata>;
export type ExplainNode = NonNullItems<coreModels.ExplainNode>;
export type ResolveResult = NonNullItems<sqlLangModels.ResolveResult>;
export type SearchResultWithNodes = NonNullItems<searchModels.SearchResultWithNodes>;

/**
 * Drops null and undefined entries from every array reachable from `value`,
 * in place — which keeps the model classes' prototypes intact.
 */
export function stripNullItems<T>(value: T): NonNullItems<T> {
	strip(value);
	return value as NonNullItems<T>;
}

function strip(value: unknown): void {
	if (Array.isArray(value)) {
		for (let i = value.length - 1; i >= 0; i--) {
			if (value[i] == null) value.splice(i, 1);
			else strip(value[i]);
		}
		return;
	}

	if (value !== null && typeof value === 'object') {
		for (const nested of Object.values(value)) strip(nested);
	}
}

export const GetWorkspaceGraph = async (): Promise<WorkspaceNode | null> =>
	stripNullItems(await graphService.GetWorkspaceGraph());

export const GetDBInstanceNodeByID = async (
	dbInstanceID: string
): Promise<DBInstanceNode | null> =>
	stripNullItems(await graphService.GetDBInstanceNodeByID(dbInstanceID));

export const SearchWithNodes = async (
	params: searchModels.SearchParams
): Promise<SearchResultWithNodes | null> =>
	stripNullItems(await searchService.SearchWithNodes(params));

export const GetFileNodeByID = async (fileID: string): Promise<FileNode | null> =>
	stripNullItems(await graphService.GetFileNodeByID(fileID));

export const FindDbItemNodeById = async (
	dbInstanceID: string,
	nodeID: string
): Promise<DBInstanceItemNode | null> =>
	stripNullItems(await graphService.FindDbItemNodeById(dbInstanceID, nodeID));

// Query results are graph data too: they carry the explain plan, whose nodes
// have a nullable child slice.
export const Query = async (params: dbClientModels.QueryParams): Promise<QueryResult> =>
	stripNullItems(await dbClientService.Query(params));

export const GetResultPage = async (
	params: dbClientModels.GetResultPageParams
): Promise<QueryResult> => stripNullItems(await dbClientService.GetResultPage(params));

export const LookupForeignKey = async (
	params: dbClientModels.LookupForeignKeyParams
): Promise<QueryResult> => stripNullItems(await dbClientService.LookupForeignKey(params));

export const Explain = async (params: dbClientModels.ExplainParams): Promise<ExplainResult> =>
	stripNullItems(await dbClientService.Explain(params));

export const Plan = async (params: dbClientModels.PlanParams): Promise<ExplainResult> =>
	stripNullItems(await dbClientService.Plan(params));

export const Resolve = async (params: sqlLangModels.PositionParams): Promise<ResolveResult> =>
	stripNullItems(await sqlLangService.Resolve(params));

/**
 * Nodes the app synthesises itself — git placeholders, search hits, temp files
 * — built through the generated classes so they pick up the same defaults.
 */
export const newFileNode = (source: Partial<FileNode>): FileNode =>
	stripNullItems(new models.FileNode(source));

export const newFolderNode = (source: Partial<FolderNode>): FolderNode =>
	stripNullItems(new models.FolderNode(source));
