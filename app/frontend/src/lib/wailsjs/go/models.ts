export namespace apikey {
	
	export class APIKeyRole {
	    id: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new APIKeyRole(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	    }
	}
	export class APIKeyEntry {
	    id: string;
	    name?: string;
	    prefix?: string;
	    roles: APIKeyRole[];
	    created_by?: string;
	    created_at?: string;
	    last_used_at?: string;
	    expires_at?: string;
	
	    static createFrom(source: any = {}) {
	        return new APIKeyEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.prefix = source["prefix"];
	        this.roles = this.convertValues(source["roles"], APIKeyRole);
	        this.created_by = source["created_by"];
	        this.created_at = source["created_at"];
	        this.last_used_at = source["last_used_at"];
	        this.expires_at = source["expires_at"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class CreateParams {
	    name: string;
	    role_ids: string[];
	    expires_at?: string;
	
	    static createFrom(source: any = {}) {
	        return new CreateParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.role_ids = source["role_ids"];
	        this.expires_at = source["expires_at"];
	    }
	}
	export class CreateResult {
	    id: string;
	    prefix: string;
	    key: string;
	
	    static createFrom(source: any = {}) {
	        return new CreateResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.prefix = source["prefix"];
	        this.key = source["key"];
	    }
	}

}

export namespace auth {
	
	export class DeviceCodeResponse {
	    device_code: string;
	    user_code: string;
	    verification_uri: string;
	    expires_in: number;
	    interval: number;
	
	    static createFrom(source: any = {}) {
	        return new DeviceCodeResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.device_code = source["device_code"];
	        this.user_code = source["user_code"];
	        this.verification_uri = source["verification_uri"];
	        this.expires_in = source["expires_in"];
	        this.interval = source["interval"];
	    }
	}

}

export namespace core {
	
	export class CacheStats {
	    sharedHitBlocks?: number;
	    sharedReadBlocks?: number;
	    sharedDirtiedBlocks?: number;
	    sharedWrittenBlocks?: number;
	    tempReadBlocks?: number;
	    tempWriteBlocks?: number;
	    localHitBlocks?: number;
	    localReadBlocks?: number;
	    localDirtiedBlocks?: number;
	    localWrittenBlocks?: number;
	    sharedHitBlocksFormatted?: string;
	    sharedReadBlocksFormatted?: string;
	    tempReadBlocksFormatted?: string;
	    tempWriteBlocksFormatted?: string;
	
	    static createFrom(source: any = {}) {
	        return new CacheStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sharedHitBlocks = source["sharedHitBlocks"];
	        this.sharedReadBlocks = source["sharedReadBlocks"];
	        this.sharedDirtiedBlocks = source["sharedDirtiedBlocks"];
	        this.sharedWrittenBlocks = source["sharedWrittenBlocks"];
	        this.tempReadBlocks = source["tempReadBlocks"];
	        this.tempWriteBlocks = source["tempWriteBlocks"];
	        this.localHitBlocks = source["localHitBlocks"];
	        this.localReadBlocks = source["localReadBlocks"];
	        this.localDirtiedBlocks = source["localDirtiedBlocks"];
	        this.localWrittenBlocks = source["localWrittenBlocks"];
	        this.sharedHitBlocksFormatted = source["sharedHitBlocksFormatted"];
	        this.sharedReadBlocksFormatted = source["sharedReadBlocksFormatted"];
	        this.tempReadBlocksFormatted = source["tempReadBlocksFormatted"];
	        this.tempWriteBlocksFormatted = source["tempWriteBlocksFormatted"];
	    }
	}
	export class ForeignKeyRef {
	    SchemaName: string;
	    TableName: string;
	    ColumnName: string;
	
	    static createFrom(source: any = {}) {
	        return new ForeignKeyRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.SchemaName = source["SchemaName"];
	        this.TableName = source["TableName"];
	        this.ColumnName = source["ColumnName"];
	    }
	}
	export class Column {
	    Name: string;
	    Type: string;
	    Nullable: boolean;
	    Default?: string;
	    IsPrimaryKey: boolean;
	    IsForeignKey: boolean;
	    ForeignKey?: ForeignKeyRef;
	    EnumValues: string[];
	    Extra: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new Column(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.Type = source["Type"];
	        this.Nullable = source["Nullable"];
	        this.Default = source["Default"];
	        this.IsPrimaryKey = source["IsPrimaryKey"];
	        this.IsForeignKey = source["IsForeignKey"];
	        this.ForeignKey = this.convertValues(source["ForeignKey"], ForeignKeyRef);
	        this.EnumValues = source["EnumValues"];
	        this.Extra = source["Extra"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ExplainNode {
	    id: string;
	    type: string;
	    operation: string;
	    targetTable?: string;
	    indexName?: string;
	    condition?: string;
	    sortKey?: string[];
	    estimatedRows?: number;
	    estimatedCost?: number;
	    exclusiveCost?: number;
	    actualRows?: number;
	    actualTime?: number;
	    exclusiveTime?: number;
	    peakMemory?: number;
	    children: ExplainNode[];
	    metadata: Record<string, any>;
	    rawOutput?: string;
	    depth: number;
	    percentOfTotal: number;
	    warnings: string[];
	    cacheStats?: CacheStats;
	
	    static createFrom(source: any = {}) {
	        return new ExplainNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.operation = source["operation"];
	        this.targetTable = source["targetTable"];
	        this.indexName = source["indexName"];
	        this.condition = source["condition"];
	        this.sortKey = source["sortKey"];
	        this.estimatedRows = source["estimatedRows"];
	        this.estimatedCost = source["estimatedCost"];
	        this.exclusiveCost = source["exclusiveCost"];
	        this.actualRows = source["actualRows"];
	        this.actualTime = source["actualTime"];
	        this.exclusiveTime = source["exclusiveTime"];
	        this.peakMemory = source["peakMemory"];
	        this.children = this.convertValues(source["children"], ExplainNode);
	        this.metadata = source["metadata"];
	        this.rawOutput = source["rawOutput"];
	        this.depth = source["depth"];
	        this.percentOfTotal = source["percentOfTotal"];
	        this.warnings = source["warnings"];
	        this.cacheStats = this.convertValues(source["cacheStats"], CacheStats);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class Function {
	    schema: string;
	    name: string;
	    args: string;
	    result: string;
	    kind: string;
	    description?: string;
	    oid: number;
	
	    static createFrom(source: any = {}) {
	        return new Function(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema = source["schema"];
	        this.name = source["name"];
	        this.args = source["args"];
	        this.result = source["result"];
	        this.kind = source["kind"];
	        this.description = source["description"];
	        this.oid = source["oid"];
	    }
	}
	export class IndexColumnInfo {
	    Name: string;
	    Position: number;
	    Collation: string;
	    Descending: boolean;
	
	    static createFrom(source: any = {}) {
	        return new IndexColumnInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.Position = source["Position"];
	        this.Collation = source["Collation"];
	        this.Descending = source["Descending"];
	    }
	}
	export class IndexInfo {
	    Name: string;
	    TableName: string;
	    DDL: string;
	    Columns: IndexColumnInfo[];
	
	    static createFrom(source: any = {}) {
	        return new IndexInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.TableName = source["TableName"];
	        this.DDL = source["DDL"];
	        this.Columns = this.convertValues(source["Columns"], IndexColumnInfo);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class InspectField {
	    Name: string;
	    Alias?: string;
	    Table: string;
	    Schema: string;
	    StartLine: number;
	    StartCol: number;
	    EndCol: number;
	
	    static createFrom(source: any = {}) {
	        return new InspectField(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.Alias = source["Alias"];
	        this.Table = source["Table"];
	        this.Schema = source["Schema"];
	        this.StartLine = source["StartLine"];
	        this.StartCol = source["StartCol"];
	        this.EndCol = source["EndCol"];
	    }
	}
	export class InspectTable {
	    Name: string;
	    Alias?: string;
	    Schema: string;
	    StartLine: number;
	    StartCol: number;
	    EndCol: number;
	
	    static createFrom(source: any = {}) {
	        return new InspectTable(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.Alias = source["Alias"];
	        this.Schema = source["Schema"];
	        this.StartLine = source["StartLine"];
	        this.StartCol = source["StartCol"];
	        this.EndCol = source["EndCol"];
	    }
	}
	export class InspectStatement {
	    Operation: string;
	    Fields: InspectField[];
	    Tables: InspectTable[];
	    Where: InspectField[];
	    Subqueries: InspectStatement[];
	
	    static createFrom(source: any = {}) {
	        return new InspectStatement(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Operation = source["Operation"];
	        this.Fields = this.convertValues(source["Fields"], InspectField);
	        this.Tables = this.convertValues(source["Tables"], InspectTable);
	        this.Where = this.convertValues(source["Where"], InspectField);
	        this.Subqueries = this.convertValues(source["Subqueries"], InspectStatement);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class Setting {
	    name: string;
	    description?: string;
	    value?: string;
	
	    static createFrom(source: any = {}) {
	        return new Setting(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.value = source["value"];
	    }
	}
	export class Type {
	    schema: string;
	    name: string;
	    kind: string;
	    display: string;
	    description?: string;
	    enumLabels?: string[];
	
	    static createFrom(source: any = {}) {
	        return new Type(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema = source["schema"];
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.display = source["display"];
	        this.description = source["description"];
	        this.enumLabels = source["enumLabels"];
	    }
	}
	export class TriggerInfo {
	    Name: string;
	    TableName: string;
	    DDL: string;
	
	    static createFrom(source: any = {}) {
	        return new TriggerInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.TableName = source["TableName"];
	        this.DDL = source["DDL"];
	    }
	}
	export class Table {
	    Name: string;
	    Columns: Column[];
	    PrimaryKey: string[];
	    DDL: string;
	
	    static createFrom(source: any = {}) {
	        return new Table(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.Columns = this.convertValues(source["Columns"], Column);
	        this.PrimaryKey = source["PrimaryKey"];
	        this.DDL = source["DDL"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Schema {
	    Name: string;
	    Tables: Table[];
	    ForeignTables: Table[];
	    Views: Table[];
	    MaterializedViews: Table[];
	    Indexes: IndexInfo[];
	    Triggers: TriggerInfo[];
	    Stats: Record<string, string>;
	    Types: Type[];
	    Functions: Function[];
	    Settings: Setting[];
	
	    static createFrom(source: any = {}) {
	        return new Schema(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.Tables = this.convertValues(source["Tables"], Table);
	        this.ForeignTables = this.convertValues(source["ForeignTables"], Table);
	        this.Views = this.convertValues(source["Views"], Table);
	        this.MaterializedViews = this.convertValues(source["MaterializedViews"], Table);
	        this.Indexes = this.convertValues(source["Indexes"], IndexInfo);
	        this.Triggers = this.convertValues(source["Triggers"], TriggerInfo);
	        this.Stats = source["Stats"];
	        this.Types = this.convertValues(source["Types"], Type);
	        this.Functions = this.convertValues(source["Functions"], Function);
	        this.Settings = this.convertValues(source["Settings"], Setting);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Metadata {
	    DefaultDB: string;
	    DefaultSchema: string;
	    CurrentSchema: string;
	    Schemas: Schema[];
	
	    static createFrom(source: any = {}) {
	        return new Metadata(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.DefaultDB = source["DefaultDB"];
	        this.DefaultSchema = source["DefaultSchema"];
	        this.CurrentSchema = source["CurrentSchema"];
	        this.Schemas = this.convertValues(source["Schemas"], Schema);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PermissionEntry {
	    DbInstanceID?: string;
	    SchemaName?: string;
	    TableName?: string;
	    ColumnName?: string;
	    Action: string;
	    Effect: string;
	    RoleName: string;
	
	    static createFrom(source: any = {}) {
	        return new PermissionEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.DbInstanceID = source["DbInstanceID"];
	        this.SchemaName = source["SchemaName"];
	        this.TableName = source["TableName"];
	        this.ColumnName = source["ColumnName"];
	        this.Action = source["Action"];
	        this.Effect = source["Effect"];
	        this.RoleName = source["RoleName"];
	    }
	}
	
	
	
	

}

export namespace datasource {
	
	export class GetResult {
	    name: string;
	    dsn: string;
	    ssh: string;
	    max_open_conns: number;
	    max_idle_conns: number;
	    conn_max_lifetime: number;
	    conn_max_idle_time: number;
	
	    static createFrom(source: any = {}) {
	        return new GetResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.dsn = source["dsn"];
	        this.ssh = source["ssh"];
	        this.max_open_conns = source["max_open_conns"];
	        this.max_idle_conns = source["max_idle_conns"];
	        this.conn_max_lifetime = source["conn_max_lifetime"];
	        this.conn_max_idle_time = source["conn_max_idle_time"];
	    }
	}
	export class UpsertParams {
	    id: string;
	    db_type: string;
	    name: string;
	    dsn: string;
	    ssh: string;
	    max_open_conns: number;
	    max_idle_conns: number;
	    conn_max_lifetime: number;
	    conn_max_idle_time: number;
	
	    static createFrom(source: any = {}) {
	        return new UpsertParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.db_type = source["db_type"];
	        this.name = source["name"];
	        this.dsn = source["dsn"];
	        this.ssh = source["ssh"];
	        this.max_open_conns = source["max_open_conns"];
	        this.max_idle_conns = source["max_idle_conns"];
	        this.conn_max_lifetime = source["conn_max_lifetime"];
	        this.conn_max_idle_time = source["conn_max_idle_time"];
	    }
	}

}

export namespace db_client {
	
	export class CancelQueryParams {
	    DbInstanceID: string;
	    FileID: string;
	
	    static createFrom(source: any = {}) {
	        return new CancelQueryParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.DbInstanceID = source["DbInstanceID"];
	        this.FileID = source["FileID"];
	    }
	}
	export class ExplainParams {
	    FileID: string;
	    Statement: string;
	    DbInstanceID: string;
	    FolderID: string;
	    RuntimeVars: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new ExplainParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.FileID = source["FileID"];
	        this.Statement = source["Statement"];
	        this.DbInstanceID = source["DbInstanceID"];
	        this.FolderID = source["FolderID"];
	        this.RuntimeVars = source["RuntimeVars"];
	    }
	}
	export class ExportParams {
	    FileID: string;
	    Statement: string;
	    DbInstanceID: string;
	    FolderID: string;
	    Format: string;
	    Filename: string;
	    RuntimeVars: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new ExportParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.FileID = source["FileID"];
	        this.Statement = source["Statement"];
	        this.DbInstanceID = source["DbInstanceID"];
	        this.FolderID = source["FolderID"];
	        this.Format = source["Format"];
	        this.Filename = source["Filename"];
	        this.RuntimeVars = source["RuntimeVars"];
	    }
	}
	export class GenerateSelectSQLParams {
	    databaseId: string;
	    schema: string;
	    table: string;
	    limit: number;
	
	    static createFrom(source: any = {}) {
	        return new GenerateSelectSQLParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.databaseId = source["databaseId"];
	        this.schema = source["schema"];
	        this.table = source["table"];
	        this.limit = source["limit"];
	    }
	}
	export class GenerateSelectSQLResult {
	    sql: string;
	
	    static createFrom(source: any = {}) {
	        return new GenerateSelectSQLResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sql = source["sql"];
	    }
	}
	export class TableEditInput {
	    databaseId: string;
	    schema: string;
	    table: string;
	    column: string;
	    value: string;
	    rowIndex: number;
	    columnIndex: number;
	    primaryKeyValues: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new TableEditInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.databaseId = source["databaseId"];
	        this.schema = source["schema"];
	        this.table = source["table"];
	        this.column = source["column"];
	        this.value = source["value"];
	        this.rowIndex = source["rowIndex"];
	        this.columnIndex = source["columnIndex"];
	        this.primaryKeyValues = source["primaryKeyValues"];
	    }
	}
	export class GenerateUpdateSQLParams {
	    databaseId: string;
	    edits: TableEditInput[];
	
	    static createFrom(source: any = {}) {
	        return new GenerateUpdateSQLParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.databaseId = source["databaseId"];
	        this.edits = this.convertValues(source["edits"], TableEditInput);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class GenerateUpdateSQLResult {
	    sql: string;
	
	    static createFrom(source: any = {}) {
	        return new GenerateUpdateSQLResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sql = source["sql"];
	    }
	}
	export class GetResultPageParams {
	    DbInstanceID: string;
	    FileID: string;
	    ResultID: string;
	    Page: number;
	
	    static createFrom(source: any = {}) {
	        return new GetResultPageParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.DbInstanceID = source["DbInstanceID"];
	        this.FileID = source["FileID"];
	        this.ResultID = source["ResultID"];
	        this.Page = source["Page"];
	    }
	}
	export class LookupForeignKeyParams {
	    databaseId: string;
	    schema: string;
	    table: string;
	    fkColumn: string;
	    displayColumns: string[];
	    query: string;
	    currentValue: string;
	    limit: number;
	
	    static createFrom(source: any = {}) {
	        return new LookupForeignKeyParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.databaseId = source["databaseId"];
	        this.schema = source["schema"];
	        this.table = source["table"];
	        this.fkColumn = source["fkColumn"];
	        this.displayColumns = source["displayColumns"];
	        this.query = source["query"];
	        this.currentValue = source["currentValue"];
	        this.limit = source["limit"];
	    }
	}
	export class PingParams {
	    DbInstanceID: string;
	    db_type: string;
	    dsn: string;
	    folder_id: string;
	    ssh?: graph.DBInstanceSSHConfig;
	    proxified: boolean;
	    no_cache?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PingParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.DbInstanceID = source["DbInstanceID"];
	        this.db_type = source["db_type"];
	        this.dsn = source["dsn"];
	        this.folder_id = source["folder_id"];
	        this.ssh = this.convertValues(source["ssh"], graph.DBInstanceSSHConfig);
	        this.proxified = source["proxified"];
	        this.no_cache = source["no_cache"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PlanParams {
	    FileID: string;
	    Statement: string;
	    DbInstanceID: string;
	    FolderID: string;
	    RuntimeVars: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new PlanParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.FileID = source["FileID"];
	        this.Statement = source["Statement"];
	        this.DbInstanceID = source["DbInstanceID"];
	        this.FolderID = source["FolderID"];
	        this.RuntimeVars = source["RuntimeVars"];
	    }
	}
	export class QueryParams {
	    FileID: string;
	    Statement: string;
	    DbInstanceID: string;
	    FolderID: string;
	    ForExport: boolean;
	    RuntimeVars: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new QueryParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.FileID = source["FileID"];
	        this.Statement = source["Statement"];
	        this.DbInstanceID = source["DbInstanceID"];
	        this.FolderID = source["FolderID"];
	        this.ForExport = source["ForExport"];
	        this.RuntimeVars = source["RuntimeVars"];
	    }
	}
	export class QuerySchemaParams {
	    DatabaseInstanceID: string;
	    NoCache: boolean;
	
	    static createFrom(source: any = {}) {
	        return new QuerySchemaParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.DatabaseInstanceID = source["DatabaseInstanceID"];
	        this.NoCache = source["NoCache"];
	    }
	}
	export class StartQueryParams {
	    FileID: string;
	    Statement: string;
	    DbInstanceID: string;
	    FolderID: string;
	    RuntimeVars: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new StartQueryParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.FileID = source["FileID"];
	        this.Statement = source["Statement"];
	        this.DbInstanceID = source["DbInstanceID"];
	        this.FolderID = source["FolderID"];
	        this.RuntimeVars = source["RuntimeVars"];
	    }
	}
	export class StartQueryResult {
	    executionId: string;
	    dbInstanceId: string;
	    fileId: string;
	    errors?: string[];
	
	    static createFrom(source: any = {}) {
	        return new StartQueryResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.executionId = source["executionId"];
	        this.dbInstanceId = source["dbInstanceId"];
	        this.fileId = source["fileId"];
	        this.errors = source["errors"];
	    }
	}

}

export namespace db_types {
	
	export class JSONNullString {
	    String: string;
	    Valid: boolean;
	
	    static createFrom(source: any = {}) {
	        return new JSONNullString(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.String = source["String"];
	        this.Valid = source["Valid"];
	    }
	}

}

export namespace fs_provider {
	
	export class CommandResult {
	    stdout: string;
	    stderr: string;
	    exitCode: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new CommandResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stdout = source["stdout"];
	        this.stderr = source["stderr"];
	        this.exitCode = source["exitCode"];
	        this.error = source["error"];
	    }
	}
	export class DeleteParams {
	    uri: string;
	    recursive: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DeleteParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uri = source["uri"];
	        this.recursive = source["recursive"];
	    }
	}
	export class DirEntry {
	    Name: string;
	    Type: number;
	
	    static createFrom(source: any = {}) {
	        return new DirEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.Type = source["Type"];
	    }
	}
	export class ExecuteCommandParams {
	    workspaceId: string;
	    command: string;
	    args: string[];
	    workingDir?: string;
	    timeout?: number;
	
	    static createFrom(source: any = {}) {
	        return new ExecuteCommandParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaceId = source["workspaceId"];
	        this.command = source["command"];
	        this.args = source["args"];
	        this.workingDir = source["workingDir"];
	        this.timeout = source["timeout"];
	    }
	}
	export class FileStat {
	    Type: number;
	    Size: number;
	    // Go type: time
	    Mtime: any;
	
	    static createFrom(source: any = {}) {
	        return new FileStat(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Type = source["Type"];
	        this.Size = source["Size"];
	        this.Mtime = this.convertValues(source["Mtime"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class MkdirParams {
	    uri: string;
	
	    static createFrom(source: any = {}) {
	        return new MkdirParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uri = source["uri"];
	    }
	}
	export class ReadDirectoryParams {
	    uri: string;
	
	    static createFrom(source: any = {}) {
	        return new ReadDirectoryParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uri = source["uri"];
	    }
	}
	export class ReadFileParams {
	    uri: string;
	    startLine?: number;
	    endLine?: number;
	
	    static createFrom(source: any = {}) {
	        return new ReadFileParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uri = source["uri"];
	        this.startLine = source["startLine"];
	        this.endLine = source["endLine"];
	    }
	}
	export class RenameParams {
	    old_uri: string;
	    new_uri: string;
	
	    static createFrom(source: any = {}) {
	        return new RenameParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.old_uri = source["old_uri"];
	        this.new_uri = source["new_uri"];
	    }
	}
	export class WriteParams {
	    uri: string;
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new WriteParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uri = source["uri"];
	        this.content = source["content"];
	    }
	}

}

export namespace generated {
	
	export class Group {
	    id: string;
	    workspace_id: string;
	    name: string;
	    source: string;
	    external_id: db_types.JSONNullString;
	    // Go type: time
	    updated_at: any;
	    deleted_at: sql.NullTime;
	
	    static createFrom(source: any = {}) {
	        return new Group(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.workspace_id = source["workspace_id"];
	        this.name = source["name"];
	        this.source = source["source"];
	        this.external_id = this.convertValues(source["external_id"], db_types.JSONNullString);
	        this.updated_at = this.convertValues(source["updated_at"], null);
	        this.deleted_at = this.convertValues(source["deleted_at"], sql.NullTime);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ListGroupsByWorkspaceRow {
	    id: string;
	    workspace_id: string;
	    name: string;
	    member_count: number;
	    role_count: number;
	
	    static createFrom(source: any = {}) {
	        return new ListGroupsByWorkspaceRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.workspace_id = source["workspace_id"];
	        this.name = source["name"];
	        this.member_count = source["member_count"];
	        this.role_count = source["role_count"];
	    }
	}
	export class ListRolesByWorkspaceRow {
	    id: string;
	    workspace_id: string;
	    name: string;
	    user_count: number;
	    permission_count: number;
	    group_count: number;
	
	    static createFrom(source: any = {}) {
	        return new ListRolesByWorkspaceRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.workspace_id = source["workspace_id"];
	        this.name = source["name"];
	        this.user_count = source["user_count"];
	        this.permission_count = source["permission_count"];
	        this.group_count = source["group_count"];
	    }
	}
	export class MutationCommit {
	    id: string;
	    // Go type: time
	    created_at: any;
	    operation: string;
	    table_name: string;
	    object_id: string;
	    payload: any;
	    user_id: string;
	    workspace_id: string;
	
	    static createFrom(source: any = {}) {
	        return new MutationCommit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.operation = source["operation"];
	        this.table_name = source["table_name"];
	        this.object_id = source["object_id"];
	        this.payload = source["payload"];
	        this.user_id = source["user_id"];
	        this.workspace_id = source["workspace_id"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Role {
	    id: string;
	    workspace_id: string;
	    name: string;
	    // Go type: time
	    updated_at: any;
	    deleted_at: sql.NullTime;
	
	    static createFrom(source: any = {}) {
	        return new Role(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.workspace_id = source["workspace_id"];
	        this.name = source["name"];
	        this.updated_at = this.convertValues(source["updated_at"], null);
	        this.deleted_at = this.convertValues(source["deleted_at"], sql.NullTime);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class User {
	    id: string;
	    name: db_types.JSONNullString;
	    email: db_types.JSONNullString;
	    avatar_url: db_types.JSONNullString;
	
	    static createFrom(source: any = {}) {
	        return new User(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = this.convertValues(source["name"], db_types.JSONNullString);
	        this.email = this.convertValues(source["email"], db_types.JSONNullString);
	        this.avatar_url = this.convertValues(source["avatar_url"], db_types.JSONNullString);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Workspace {
	    id: string;
	    name: string;
	    git_remote_url: db_types.JSONNullString;
	    last_pulled_at: sql.NullTime;
	    owner_id: db_types.JSONNullString;
	    statement_timeout_ms: number;
	    max_result_size_mb: number;
	
	    static createFrom(source: any = {}) {
	        return new Workspace(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.git_remote_url = this.convertValues(source["git_remote_url"], db_types.JSONNullString);
	        this.last_pulled_at = this.convertValues(source["last_pulled_at"], sql.NullTime);
	        this.owner_id = this.convertValues(source["owner_id"], db_types.JSONNullString);
	        this.statement_timeout_ms = source["statement_timeout_ms"];
	        this.max_result_size_mb = source["max_result_size_mb"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace git {
	
	export class BranchInfo {
	    name: string;
	    isCurrent: boolean;
	    isRemote: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BranchInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.isCurrent = source["isCurrent"];
	        this.isRemote = source["isRemote"];
	    }
	}
	export class CommitParams {
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new CommitParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.message = source["message"];
	    }
	}
	export class GetFileDiffContentParams {
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new GetFileDiffContentParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	    }
	}
	export class GetFileDiffContentResult {
	    leftContent: string;
	    rightContent: string;
	
	    static createFrom(source: any = {}) {
	        return new GetFileDiffContentResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.leftContent = source["leftContent"];
	        this.rightContent = source["rightContent"];
	    }
	}
	export class GitFileStatusItem {
	    path: string;
	    status: string;
	    porcelainCode: string;
	
	    static createFrom(source: any = {}) {
	        return new GitFileStatusItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.status = source["status"];
	        this.porcelainCode = source["porcelainCode"];
	    }
	}
	export class GitFileStatus {
	    branch: string;
	    staged: GitFileStatusItem[];
	    unstaged: GitFileStatusItem[];
	    untracked: GitFileStatusItem[];
	    hasChanges: boolean;
	    commitsAhead: number;
	    commitsBehind: number;
	
	    static createFrom(source: any = {}) {
	        return new GitFileStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.branch = source["branch"];
	        this.staged = this.convertValues(source["staged"], GitFileStatusItem);
	        this.unstaged = this.convertValues(source["unstaged"], GitFileStatusItem);
	        this.untracked = this.convertValues(source["untracked"], GitFileStatusItem);
	        this.hasChanges = source["hasChanges"];
	        this.commitsAhead = source["commitsAhead"];
	        this.commitsBehind = source["commitsBehind"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class GitWorkspaceStatus {
	    gitAvailable: boolean;
	    isGitRepo: boolean;
	    hasRemote: boolean;
	    remoteUrl?: string;
	    configuredRemoteUrl?: string;
	
	    static createFrom(source: any = {}) {
	        return new GitWorkspaceStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gitAvailable = source["gitAvailable"];
	        this.isGitRepo = source["isGitRepo"];
	        this.hasRemote = source["hasRemote"];
	        this.remoteUrl = source["remoteUrl"];
	        this.configuredRemoteUrl = source["configuredRemoteUrl"];
	    }
	}
	export class InitAndPublishParams {
	    repoName: string;
	    visibility: string;
	    remoteUrl?: string;
	
	    static createFrom(source: any = {}) {
	        return new InitAndPublishParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.repoName = source["repoName"];
	        this.visibility = source["visibility"];
	        this.remoteUrl = source["remoteUrl"];
	    }
	}
	export class LinkExistingParams {
	    remoteUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new LinkExistingParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.remoteUrl = source["remoteUrl"];
	    }
	}
	export class LinkStatus {
	    scenario: string;
	    branch: string;
	
	    static createFrom(source: any = {}) {
	        return new LinkStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scenario = source["scenario"];
	        this.branch = source["branch"];
	    }
	}
	export class ReconcileResult {
	    action: string;
	    changed: boolean;
	    remoteUrl: string;
	    backupPath: string;
	    backupRef: string;
	
	    static createFrom(source: any = {}) {
	        return new ReconcileResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.action = source["action"];
	        this.changed = source["changed"];
	        this.remoteUrl = source["remoteUrl"];
	        this.backupPath = source["backupPath"];
	        this.backupRef = source["backupRef"];
	    }
	}
	export class RevertFileParams {
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new RevertFileParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	    }
	}
	export class StageFileParams {
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new StageFileParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	    }
	}
	export class SwitchBranchParams {
	    branchName: string;
	
	    static createFrom(source: any = {}) {
	        return new SwitchBranchParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.branchName = source["branchName"];
	    }
	}
	export class UnstageFileParams {
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new UnstageFileParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	    }
	}

}

export namespace graph {
	
	export class ColumnMetadata {
	    hasAllPrimaryKeys: boolean;
	    isPrimaryKey: boolean;
	    isForeignKey?: boolean;
	    databaseId?: string;
	    schema?: string;
	    table?: string;
	    originalColumnName?: string;
	    primaryKeys?: string[];
	    primaryKeysIdxs?: number[];
	    dataType?: string;
	    enumValues?: string[];
	
	    static createFrom(source: any = {}) {
	        return new ColumnMetadata(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hasAllPrimaryKeys = source["hasAllPrimaryKeys"];
	        this.isPrimaryKey = source["isPrimaryKey"];
	        this.isForeignKey = source["isForeignKey"];
	        this.databaseId = source["databaseId"];
	        this.schema = source["schema"];
	        this.table = source["table"];
	        this.originalColumnName = source["originalColumnName"];
	        this.primaryKeys = source["primaryKeys"];
	        this.primaryKeysIdxs = source["primaryKeysIdxs"];
	        this.dataType = source["dataType"];
	        this.enumValues = source["enumValues"];
	    }
	}
	export class EditorSnippet {
	    prefix: string;
	    body: string;
	    description?: string;
	
	    static createFrom(source: any = {}) {
	        return new EditorSnippet(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.prefix = source["prefix"];
	        this.body = source["body"];
	        this.description = source["description"];
	    }
	}
	export class Keybinding {
	    key: string;
	    command: string;
	    when?: string;
	
	    static createFrom(source: any = {}) {
	        return new Keybinding(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.command = source["command"];
	        this.when = source["when"];
	    }
	}
	export class ConfigResponse {
	    keybindings: Keybinding[];
	    editor_snippets: EditorSnippet[];
	
	    static createFrom(source: any = {}) {
	        return new ConfigResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.keybindings = this.convertValues(source["keybindings"], Keybinding);
	        this.editor_snippets = this.convertValues(source["editor_snippets"], EditorSnippet);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DBInstanceItemNode {
	    id: string;
	    uri: string;
	    type: string;
	    name: string;
	    path: string;
	    badges: string[];
	    metadata: any;
	    parent_id: string;
	    children: DBInstanceItemNode[];
	
	    static createFrom(source: any = {}) {
	        return new DBInstanceItemNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.uri = source["uri"];
	        this.type = source["type"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.badges = source["badges"];
	        this.metadata = source["metadata"];
	        this.parent_id = source["parent_id"];
	        this.children = this.convertValues(source["children"], DBInstanceItemNode);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class FolderNode {
	    id: string;
	    uri: string;
	    type: string;
	    name: string;
	    folder_id: string;
	    files: FileNode[];
	    folders: FolderNode[];
	    db_instances: DBInstanceNode[];
	    variables?: Record<string, string>;
	    badges: string[];
	
	    static createFrom(source: any = {}) {
	        return new FolderNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.uri = source["uri"];
	        this.type = source["type"];
	        this.name = source["name"];
	        this.folder_id = source["folder_id"];
	        this.files = this.convertValues(source["files"], FileNode);
	        this.folders = this.convertValues(source["folders"], FolderNode);
	        this.db_instances = this.convertValues(source["db_instances"], DBInstanceNode);
	        this.variables = source["variables"];
	        this.badges = source["badges"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ExplainResult {
	    id?: string;
	    root?: core.ExplainNode;
	    totalCost?: number;
	    raw?: string;
	    errors?: string[];
	    errorPosition?: number;
	    durationMs?: number;
	
	    static createFrom(source: any = {}) {
	        return new ExplainResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.root = this.convertValues(source["root"], core.ExplainNode);
	        this.totalCost = source["totalCost"];
	        this.raw = source["raw"];
	        this.errors = source["errors"];
	        this.errorPosition = source["errorPosition"];
	        this.durationMs = source["durationMs"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class QueryResult {
	    id?: string;
	    columns?: string[];
	    rows?: any[][];
	    affectedRows?: number;
	    rowCount: number;
	    durationMs?: number;
	    errors?: string[];
	    errorPosition?: number;
	    explain?: ExplainResult;
	    page: number;
	    pageSize: number;
	    available?: number;
	    status?: string;
	    columnMetadata?: ColumnMetadata[];
	
	    static createFrom(source: any = {}) {
	        return new QueryResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.columns = source["columns"];
	        this.rows = source["rows"];
	        this.affectedRows = source["affectedRows"];
	        this.rowCount = source["rowCount"];
	        this.durationMs = source["durationMs"];
	        this.errors = source["errors"];
	        this.errorPosition = source["errorPosition"];
	        this.explain = this.convertValues(source["explain"], ExplainResult);
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.available = source["available"];
	        this.status = source["status"];
	        this.columnMetadata = this.convertValues(source["columnMetadata"], ColumnMetadata);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DatabaseRef {
	    name: string;
	    id: string;
	
	    static createFrom(source: any = {}) {
	        return new DatabaseRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.id = source["id"];
	    }
	}
	export class FileNode {
	    id: string;
	    uri: string;
	    type: string;
	    name: string;
	    folder_id: string;
	    databases?: DatabaseRef[];
	    queryResults?: Record<string, QueryResult>;
	    planResults?: Record<string, ExplainResult>;
	    explainResults?: Record<string, ExplainResult>;
	    badges: string[];
	
	    static createFrom(source: any = {}) {
	        return new FileNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.uri = source["uri"];
	        this.type = source["type"];
	        this.name = source["name"];
	        this.folder_id = source["folder_id"];
	        this.databases = this.convertValues(source["databases"], DatabaseRef);
	        this.queryResults = this.convertValues(source["queryResults"], QueryResult, true);
	        this.planResults = this.convertValues(source["planResults"], ExplainResult, true);
	        this.explainResults = this.convertValues(source["explainResults"], ExplainResult, true);
	        this.badges = source["badges"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DBInstanceSSHConfig {
	    enabled: boolean;
	    host: string;
	    port: number;
	    user: string;
	    auth_method: string;
	    password: string;
	    private_key: string;
	    key_path: string;
	    host_key: string;
	
	    static createFrom(source: any = {}) {
	        return new DBInstanceSSHConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.user = source["user"];
	        this.auth_method = source["auth_method"];
	        this.password = source["password"];
	        this.private_key = source["private_key"];
	        this.key_path = source["key_path"];
	        this.host_key = source["host_key"];
	    }
	}
	export class DBInstanceNode {
	    id: string;
	    uri: string;
	    type: string;
	    name: string;
	    db_type: string;
	    dsn: string;
	    proxified?: boolean;
	    ssh?: DBInstanceSSHConfig;
	    folder_id: string;
	    workspace_id: string;
	    children: DBInstanceItemNode[];
	    files: FileNode[];
	    folders: FolderNode[];
	
	    static createFrom(source: any = {}) {
	        return new DBInstanceNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.uri = source["uri"];
	        this.type = source["type"];
	        this.name = source["name"];
	        this.db_type = source["db_type"];
	        this.dsn = source["dsn"];
	        this.proxified = source["proxified"];
	        this.ssh = this.convertValues(source["ssh"], DBInstanceSSHConfig);
	        this.folder_id = source["folder_id"];
	        this.workspace_id = source["workspace_id"];
	        this.children = this.convertValues(source["children"], DBInstanceItemNode);
	        this.files = this.convertValues(source["files"], FileNode);
	        this.folders = this.convertValues(source["folders"], FolderNode);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	
	
	
	
	
	export class SqlFileCandidate {
	    name: string;
	    path: string;
	    uri: string;
	    preview: string;
	
	    static createFrom(source: any = {}) {
	        return new SqlFileCandidate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.uri = source["uri"];
	        this.preview = source["preview"];
	    }
	}
	export class ThemeVariables {
	    shared: Record<string, string>;
	    light: Record<string, string>;
	    dark: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new ThemeVariables(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.shared = source["shared"];
	        this.light = source["light"];
	        this.dark = source["dark"];
	    }
	}
	export class UserNode {
	    id: string;
	    type: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new UserNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.name = source["name"];
	    }
	}
	export class VariableCandidate {
	    name: string;
	    value: string;
	    source: string;
	    source_uri: string;
	
	    static createFrom(source: any = {}) {
	        return new VariableCandidate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.value = source["value"];
	        this.source = source["source"];
	        this.source_uri = source["source_uri"];
	    }
	}
	export class WorkspaceFS {
	    WorkspaceID: string;
	    WorkspaceRoot: string;
	    RootURI: string;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceFS(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.WorkspaceID = source["WorkspaceID"];
	        this.WorkspaceRoot = source["WorkspaceRoot"];
	        this.RootURI = source["RootURI"];
	    }
	}
	export class WorkspaceNode {
	    id: string;
	    type: string;
	    name: string;
	    is_owner: boolean;
	    statement_timeout_ms: number;
	    max_result_size_mb: number;
	    user?: UserNode;
	    folders: FolderNode[];
	    db_instances: DBInstanceNode[];
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.name = source["name"];
	        this.is_owner = source["is_owner"];
	        this.statement_timeout_ms = source["statement_timeout_ms"];
	        this.max_result_size_mb = source["max_result_size_mb"];
	        this.user = this.convertValues(source["user"], UserNode);
	        this.folders = this.convertValues(source["folders"], FolderNode);
	        this.db_instances = this.convertValues(source["db_instances"], DBInstanceNode);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace group {
	
	export class GroupRoleEntry {
	    id: string;
	    name?: string;
	    group_to_role_id: string;
	
	    static createFrom(source: any = {}) {
	        return new GroupRoleEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.group_to_role_id = source["group_to_role_id"];
	    }
	}
	export class GroupUserEntry {
	    id: string;
	    name?: string;
	    user_to_group_id: string;
	
	    static createFrom(source: any = {}) {
	        return new GroupUserEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.user_to_group_id = source["user_to_group_id"];
	    }
	}

}

export namespace history {
	
	export class CreateQueryHistoryParams {
	    Dsn: string;
	    Uri: string;
	    WorkspaceID: string;
	    DbInstanceID: string;
	    Statement: string;
	    AffectedRows?: number;
	    RowCount?: number;
	    DurationMs?: number;
	    Errors: string[];
	
	    static createFrom(source: any = {}) {
	        return new CreateQueryHistoryParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Dsn = source["Dsn"];
	        this.Uri = source["Uri"];
	        this.WorkspaceID = source["WorkspaceID"];
	        this.DbInstanceID = source["DbInstanceID"];
	        this.Statement = source["Statement"];
	        this.AffectedRows = source["AffectedRows"];
	        this.RowCount = source["RowCount"];
	        this.DurationMs = source["DurationMs"];
	        this.Errors = source["Errors"];
	    }
	}
	export class HistoryEntry {
	    id: string;
	    statement: string;
	    affectedRows?: number;
	    rowCount?: number;
	    durationMs?: number;
	    errors: string[];
	    workspaceId: string;
	    dbInstanceId: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new HistoryEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.statement = source["statement"];
	        this.affectedRows = source["affectedRows"];
	        this.rowCount = source["rowCount"];
	        this.durationMs = source["durationMs"];
	        this.errors = source["errors"];
	        this.workspaceId = source["workspaceId"];
	        this.dbInstanceId = source["dbInstanceId"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class ListHistoryParams {
	    workspaceId: string;
	    limit: number;
	    offset: number;
	
	    static createFrom(source: any = {}) {
	        return new ListHistoryParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaceId = source["workspaceId"];
	        this.limit = source["limit"];
	        this.offset = source["offset"];
	    }
	}

}

export namespace role {
	
	export class AddPermissionParams {
	    role_id: string;
	    db_instance_id?: string;
	    schema_name?: string;
	    table_name?: string;
	    column_name?: string;
	    action: string;
	    effect: string;
	
	    static createFrom(source: any = {}) {
	        return new AddPermissionParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role_id = source["role_id"];
	        this.db_instance_id = source["db_instance_id"];
	        this.schema_name = source["schema_name"];
	        this.table_name = source["table_name"];
	        this.column_name = source["column_name"];
	        this.action = source["action"];
	        this.effect = source["effect"];
	    }
	}
	export class PermissionEntry {
	    id: string;
	    role_id: string;
	    workspace_id: string;
	    db_instance_id?: string;
	    schema_name?: string;
	    table_name?: string;
	    column_name?: string;
	    action: string;
	    effect: string;
	
	    static createFrom(source: any = {}) {
	        return new PermissionEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.role_id = source["role_id"];
	        this.workspace_id = source["workspace_id"];
	        this.db_instance_id = source["db_instance_id"];
	        this.schema_name = source["schema_name"];
	        this.table_name = source["table_name"];
	        this.column_name = source["column_name"];
	        this.action = source["action"];
	        this.effect = source["effect"];
	    }
	}
	export class RoleUserEntry {
	    id: string;
	    name?: string;
	    user_to_role_id: string;
	
	    static createFrom(source: any = {}) {
	        return new RoleUserEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.user_to_role_id = source["user_to_role_id"];
	    }
	}

}

export namespace search {
	
	export class ReplaceParams {
	    workspaceId: string;
	    pattern: string;
	    replacement: string;
	    useRegex: boolean;
	    caseSensitive: boolean;
	    wholeWord: boolean;
	    includePattern: string;
	    excludePattern: string;
	    filePath: string;
	    dryRun: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ReplaceParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaceId = source["workspaceId"];
	        this.pattern = source["pattern"];
	        this.replacement = source["replacement"];
	        this.useRegex = source["useRegex"];
	        this.caseSensitive = source["caseSensitive"];
	        this.wholeWord = source["wholeWord"];
	        this.includePattern = source["includePattern"];
	        this.excludePattern = source["excludePattern"];
	        this.filePath = source["filePath"];
	        this.dryRun = source["dryRun"];
	    }
	}
	export class ReplaceResult {
	    filesModified: number;
	    totalReplacements: number;
	    modifiedFiles: string[];
	
	    static createFrom(source: any = {}) {
	        return new ReplaceResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filesModified = source["filesModified"];
	        this.totalReplacements = source["totalReplacements"];
	        this.modifiedFiles = source["modifiedFiles"];
	    }
	}
	export class SearchMatch {
	    line: number;
	    column: number;
	    lineText: string;
	    matchStart: number;
	    matchEnd: number;
	
	    static createFrom(source: any = {}) {
	        return new SearchMatch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.line = source["line"];
	        this.column = source["column"];
	        this.lineText = source["lineText"];
	        this.matchStart = source["matchStart"];
	        this.matchEnd = source["matchEnd"];
	    }
	}
	export class SearchFileResult {
	    path: string;
	    matches: SearchMatch[];
	
	    static createFrom(source: any = {}) {
	        return new SearchFileResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.matches = this.convertValues(source["matches"], SearchMatch);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class SearchParams {
	    workspaceId: string;
	    pattern: string;
	    useRegex: boolean;
	    caseSensitive: boolean;
	    wholeWord: boolean;
	    includePattern: string;
	    excludePattern: string;
	
	    static createFrom(source: any = {}) {
	        return new SearchParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaceId = source["workspaceId"];
	        this.pattern = source["pattern"];
	        this.useRegex = source["useRegex"];
	        this.caseSensitive = source["caseSensitive"];
	        this.wholeWord = source["wholeWord"];
	        this.includePattern = source["includePattern"];
	        this.excludePattern = source["excludePattern"];
	    }
	}
	export class SearchResult {
	    files: SearchFileResult[];
	    totalFiles: number;
	    totalMatches: number;
	
	    static createFrom(source: any = {}) {
	        return new SearchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.files = this.convertValues(source["files"], SearchFileResult);
	        this.totalFiles = source["totalFiles"];
	        this.totalMatches = source["totalMatches"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SearchResultWithNodes {
	    resultFolder?: graph.FolderNode;
	    totalFiles: number;
	    totalMatches: number;
	
	    static createFrom(source: any = {}) {
	        return new SearchResultWithNodes(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.resultFolder = this.convertValues(source["resultFolder"], graph.FolderNode);
	        this.totalFiles = source["totalFiles"];
	        this.totalMatches = source["totalMatches"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace server {
	
	export class Manifest {
	    live: boolean;
	    backend_version: string;
	    min_app_version: string;
	    latest_app_version: string;
	    warning: string;
	
	    static createFrom(source: any = {}) {
	        return new Manifest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.live = source["live"];
	        this.backend_version = source["backend_version"];
	        this.min_app_version = source["min_app_version"];
	        this.latest_app_version = source["latest_app_version"];
	        this.warning = source["warning"];
	    }
	}

}

export namespace sql {
	
	export class DB {
	
	
	    static createFrom(source: any = {}) {
	        return new DB(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}
	export class NullTime {
	    // Go type: time
	    Time: any;
	    Valid: boolean;
	
	    static createFrom(source: any = {}) {
	        return new NullTime(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Time = this.convertValues(source["Time"], null);
	        this.Valid = source["Valid"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace sqllang {
	
	export class CompletionCandidate {
	    Type: number;
	    Text: string;
	    InsertText: string;
	    Definition: string;
	    Comment: string;
	    kind: number;
	
	    static createFrom(source: any = {}) {
	        return new CompletionCandidate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Type = source["Type"];
	        this.Text = source["Text"];
	        this.InsertText = source["InsertText"];
	        this.Definition = source["Definition"];
	        this.Comment = source["Comment"];
	        this.kind = source["kind"];
	    }
	}
	export class CompleteResult {
	    id: string;
	    candidates: CompletionCandidate[];
	    errors: string[];
	
	    static createFrom(source: any = {}) {
	        return new CompleteResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.candidates = this.convertValues(source["candidates"], CompletionCandidate);
	        this.errors = source["errors"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class HoverResult {
	    markdown: string;
	
	    static createFrom(source: any = {}) {
	        return new HoverResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.markdown = source["markdown"];
	    }
	}
	export class LintDiagnostic {
	    ruleId: string;
	    severity: number;
	    message: string;
	    startLine: number;
	    startCol: number;
	    endLine: number;
	    endCol: number;
	
	    static createFrom(source: any = {}) {
	        return new LintDiagnostic(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ruleId = source["ruleId"];
	        this.severity = source["severity"];
	        this.message = source["message"];
	        this.startLine = source["startLine"];
	        this.startCol = source["startCol"];
	        this.endLine = source["endLine"];
	        this.endCol = source["endCol"];
	    }
	}
	export class LintResult {
	    id: string;
	    diagnostics: LintDiagnostic[];
	    errors: string[];
	
	    static createFrom(source: any = {}) {
	        return new LintResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.diagnostics = this.convertValues(source["diagnostics"], LintDiagnostic);
	        this.errors = source["errors"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PositionParams {
	    DbInstanceID: string;
	    FileID: string;
	    SQL: string;
	    Line: number;
	    Column: number;
	
	    static createFrom(source: any = {}) {
	        return new PositionParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.DbInstanceID = source["DbInstanceID"];
	        this.FileID = source["FileID"];
	        this.SQL = source["SQL"];
	        this.Line = source["Line"];
	        this.Column = source["Column"];
	    }
	}
	export class ResolveResult {
	    node?: graph.DBInstanceItemNode;
	    found: boolean;
	    kind: string;
	
	    static createFrom(source: any = {}) {
	        return new ResolveResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.node = this.convertValues(source["node"], graph.DBInstanceItemNode);
	        this.found = source["found"];
	        this.kind = source["kind"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace system {
	
	export class ExecutionLimitsResponse {
	    statement_timeout_ms: number;
	    max_result_size_mb: number;
	
	    static createFrom(source: any = {}) {
	        return new ExecutionLimitsResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.statement_timeout_ms = source["statement_timeout_ms"];
	        this.max_result_size_mb = source["max_result_size_mb"];
	    }
	}

}

export namespace terminal {
	
	export class ShellOption {
	    path: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new ShellOption(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	    }
	}

}

export namespace tokenanalyzer {
	
	export class CustomRule {
	    id: string;
	    severity: string;
	    message: string;
	    pattern: string;
	
	    static createFrom(source: any = {}) {
	        return new CustomRule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.severity = source["severity"];
	        this.message = source["message"];
	        this.pattern = source["pattern"];
	    }
	}
	export class LintRuleConfig {
	    Severity: string;
	
	    static createFrom(source: any = {}) {
	        return new LintRuleConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Severity = source["Severity"];
	    }
	}
	export class LintConfigEntry {
	    files?: string[];
	    ignores?: string[];
	    rules?: Record<string, LintRuleConfig>;
	    custom?: CustomRule[];
	
	    static createFrom(source: any = {}) {
	        return new LintConfigEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.files = source["files"];
	        this.ignores = source["ignores"];
	        this.rules = this.convertValues(source["rules"], LintRuleConfig, true);
	        this.custom = this.convertValues(source["custom"], CustomRule);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace workspace {
	
	export class CreateWorkspaceParams {
	    ID: string;
	    WorkspaceToUserID: string;
	    UserID: string;
	    Name: string;
	
	    static createFrom(source: any = {}) {
	        return new CreateWorkspaceParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.WorkspaceToUserID = source["WorkspaceToUserID"];
	        this.UserID = source["UserID"];
	        this.Name = source["Name"];
	    }
	}
	export class SearchUserResult {
	    found: boolean;
	    user_id?: string;
	    name?: string;
	    email: string;
	    already_added: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SearchUserResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.found = source["found"];
	        this.user_id = source["user_id"];
	        this.name = source["name"];
	        this.email = source["email"];
	        this.already_added = source["already_added"];
	    }
	}
	export class SetOrCreateCurrentWorkspaceParams {
	    UserID: string;
	
	    static createFrom(source: any = {}) {
	        return new SetOrCreateCurrentWorkspaceParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.UserID = source["UserID"];
	    }
	}
	export class WorkspaceUserGroup {
	    id: string;
	    name: string;
	    user_to_group_id: string;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceUserGroup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.user_to_group_id = source["user_to_group_id"];
	    }
	}
	export class WorkspaceUserRole {
	    id: string;
	    name: string;
	    user_to_role_id: string;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceUserRole(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.user_to_role_id = source["user_to_role_id"];
	    }
	}
	export class WorkspaceUserEntry {
	    id: string;
	    name?: string;
	    email?: string;
	    roles: WorkspaceUserRole[];
	    groups: WorkspaceUserGroup[];
	    is_owner: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceUserEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.email = source["email"];
	        this.roles = this.convertValues(source["roles"], WorkspaceUserRole);
	        this.groups = this.convertValues(source["groups"], WorkspaceUserGroup);
	        this.is_owner = source["is_owner"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class WorkspaceWithCurrent {
	    id: string;
	    name: string;
	    current: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceWithCurrent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.current = source["current"];
	    }
	}

}

