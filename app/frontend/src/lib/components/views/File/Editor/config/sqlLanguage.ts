const sqlKeywords = [
	// --- DML (Data Manipulation Language) ---
	'select',
	'insert',
	'as',
	'update',
	'delete',
	'merge', // SQL Server, Oracle, PostgreSQL 15+

	// --- DDL (Data Definition Language) ---
	'create',
	'alter',
	'drop',
	'cascade',
	'truncate',
	'rename',
	'view',
	'table',
	'comment', // PostgreSQL, Oracle

	// --- DCL (Data Control Language) ---
	'grant',
	'revoke',
	'deny', // SQL Server

	// --- TCL (Transaction Control Language) ---
	'begin',
	'transaction',
	'commit',
	'rollback',
	'savepoint',
	'release',
	'set transaction',
	'set session characteristics', // SQL standard

	// --- Clauses ---
	'from',
	'where',
	'group',
	'by',
	'having',
	'order',
	'limit',
	'offset',
	'fetch', // SQL:2008 FETCH FIRST...ROWS ONLY
	'only', // PostgreSQL: SELECT ONLY
	'top', // SQL Server, Sybase
	'percent', // SQL Server TOP PERCENT

	// --- Joins ---
	'join',
	'inner',
	'left',
	'right',
	'full',
	'outer',
	'cross',
	'natural',
	'using',
	'on',
	'union',
	'intersect',
	'except',
	'apply', // SQL Server: CROSS APPLY, OUTER APPLY

	// --- Insert/Update Helpers ---
	'into',
	'values',
	'set',
	'returning', // PostgreSQL, SQLite, Oracle 12c+, SQL Server 2005+

	// --- Filters & Predicates ---
	'distinct',
	'exists',
	'in',
	'like',
	'ilike', // PostgreSQL
	'similar to', // PostgreSQL
	'between',
	'is',
	'null',
	'not',
	'and',
	'or',
	'any',
	'all',
	'some',
	'overlaps', // SQL standard

	// --- CTEs and Subqueries ---
	'with',
	'recursive',
	'lateral',
	'materialized', // PostgreSQL: WITH MATERIALIZED

	// --- Conditional Expressions ---
	'case',
	'when',
	'then',
	'else',
	'end',
	'coalesce',
	'nullif',

	// --- Sorting ---
	'asc',
	'desc',
	'collate',

	// --- Constraints & Keys ---
	'check',
	'default',
	'unique',
	'primary',
	'foreign',
	'key',
	'references',
	'constraint',
	'deferrable',
	'initially deferred',
	'initially immediate',

	// --- Indexing ---
	'index',
	'using', // Also used in JOINs and CREATE INDEX
	'cluster', // PostgreSQL
	'include', // SQL Server, PostgreSQL
	'concurrently', // PostgreSQL

	// --- Type Casting & Conversions ---
	'cast',
	'convert', // SQL Server, MySQL
	'type',
	'collation',

	// --- Window Functions ---
	'window',
	'over',
	'partition',
	'row',
	'rows',
	'range',
	'groups', // SQL:2011
	'preceding',
	'following',
	'current',
	'first',
	'last',
	'exclude', // SQL:2011 (EXCLUDE CURRENT ROW etc.)

	// --- Procedural & Control Flow (SQL Server, PostgreSQL PL/pgSQL) ---
	'if',
	'then',
	'elsif',
	'elseif',
	'loop',
	'while',
	'repeat', // MySQL
	'until',
	'for',
	'each',
	'declare',
	'begin',
	'end',
	'raise',
	'return',
	'exit',
	'continue',
	'goto', // SQL Server
	'open',
	'fetch',
	'close',
	'execute',
	'perform', // PostgreSQL
	'assert',

	// --- Other Keywords ---
	'temporary',
	'temp',
	'unlogged', // PostgreSQL
	'if exists',
	'if not exists',
	'procedure',
	'function',
	'language',
	'security definer',
	'security invoker',
	'volatile',
	'stable',
	'immutable',
	'strict',
	'returns',
	'raise',
	'notice',
	'warning',
	'exception',
	'analyze',
	'vacuum',
	'explain',
	'describe',
	'show',
	'use',
	'load',
	'lock',
	'flush',
	'reindex',
	'discard',
	'listen',
	'notify',
	'unlisten',
	'checkpoint',
	'reset',
	'pragma'
];

const sqlTypes = [
	// --- Numeric types ---
	'int',
	'integer',
	'smallint',
	'bigint',
	'tinyint', // MySQL, SQL Server
	'mediumint', // MySQL
	'decimal',
	'dec',
	'numeric',
	'real',
	'float',
	'float4', // PostgreSQL alias
	'float8', // PostgreSQL alias
	'double',
	'double precision',
	'money', // PostgreSQL

	// Serial / Auto-incrementing
	'serial', // PostgreSQL
	'serial2', // PostgreSQL
	'serial4', // PostgreSQL
	'serial8', // PostgreSQL
	'bigserial', // PostgreSQL
	'identity', // SQL:2003 standard
	'generated always as identity', // SQL standard (PostgreSQL ≥10)
	'autoincrement', // SQLite keyword
	'auto_increment', // MySQL keyword

	// --- Character types ---
	'char',
	'character',
	'varchar',
	'character varying',
	'text',
	'tinytext', // MySQL
	'mediumtext', // MySQL
	'longtext', // MySQL
	'clob', // Oracle, DB2

	// --- Binary / Blob types ---
	'bytea', // PostgreSQL
	'blob',
	'tinyblob', // MySQL
	'mediumblob', // MySQL
	'longblob', // MySQL
	'binary',
	'varbinary',
	'image', // SQL Server
	'raw', // Oracle
	'long raw', // Oracle
	'bfile', // Oracle external binary file

	// --- Boolean ---
	'boolean',
	'bool', // PostgreSQL alias
	'bit', // SQL Server

	// --- Date & Time types ---
	'date',
	'time',
	'time without time zone', // PostgreSQL
	'time with time zone', // PostgreSQL
	'timetz', // PostgreSQL alias
	'timestamp',
	'timestamp without time zone', // PostgreSQL
	'timestamp with time zone',
	'timestamptz', // PostgreSQL alias
	'datetime', // MySQL, SQL Server
	'smalldatetime', // SQL Server
	'datetime2', // SQL Server
	'datetimeoffset', // SQL Server
	'year', // MySQL
	'interval', // PostgreSQL, Oracle

	// --- JSON / XML ---
	'json',
	'jsonb', // PostgreSQL
	'json_array', // SQL Server 2022
	'json_object', // SQL Server 2022
	'xml',

	// --- UUID / Identifiers ---
	'uuid', // PostgreSQL
	'uniqueidentifier', // SQL Server
	'guid', // unofficial/legacy alias

	// --- Network types (PostgreSQL only) ---
	'inet',
	'cidr',
	'macaddr',
	'macaddr8',

	// --- Spatial / Geometric types ---
	'point',
	'line',
	'lseg',
	'box',
	'path',
	'polygon',
	'circle',
	'geometry', // PostGIS, MySQL
	'geography', // PostGIS

	// --- Full-text search (PostgreSQL only) ---
	'tsvector',
	'tsquery',

	// --- Enumerated and set types ---
	'enum', // PostgreSQL, MySQL
	'set', // MySQL

	// --- Monetary ---
	'money', // PostgreSQL
	'smallmoney', // SQL Server

	// --- SQL Server-specific ---
	'nchar',
	'nvarchar',
	'ntext',
	'sql_variant',
	'hierarchyid',
	'xml', // Also in standard
	'geometry', // Spatial (SQL Server)
	'geography', // Spatial (SQL Server)

	// --- Oracle-specific ---
	'number',
	'float', // Alias for number
	'binary_float',
	'binary_double',
	'timestamp with local time zone',
	'timestamp with time zone',
	'rowid',
	'urowid',
	'varchar2',
	'nchar',
	'nvarchar2',
	'long',
	'long raw',
	'interval year to month',
	'interval day to second',
	'bfile', // external binary file

	// --- SQLite-specific (dynamic types) ---
	'integer', // maps to NUMERIC affinity
	'text',
	'real',
	'numeric',
	'blob',
	'none'
];

const sqlConstants = [
	// Boolean values
	'true',
	'false',
	'yes', // MySQL allows TRUE = YES
	'no', // MySQL allows FALSE = NO
	'on', // Often interpreted as TRUE
	'off', // Often interpreted as FALSE

	// Null value
	'null',

	// Date & Time constants
	'current_date',
	'current_time',
	'current_timestamp',
	'localtime',
	'localtimestamp',
	'now', // PostgreSQL, MySQL
	'sysdate', // Oracle, MySQL
	'systimestamp', // Oracle
	'getdate', // SQL Server
	'getutcdate', // SQL Server
	'curdate', // MySQL
	'curtime', // MySQL
	'utc_date', // MySQL
	'utc_time', // MySQL
	'utc_timestamp', // MySQL
	'current', // Oracle, sometimes shorthand for CURRENT_DATE etc.
	'today', // Informix
	'tomorrow', // Informix
	'yesterday', // Informix

	// Special numerical constants
	'infinity', // PostgreSQL (for floats and timestamps)
	'-infinity', // PostgreSQL
	'nan', // Not a number (PostgreSQL, Oracle)
	'inf', // SQLite interprets this for FLOATs

	// System/user/session constants (cross-dialect)
	'user',
	'current_user',
	'session_user',
	'system_user', // SQL Server
	'current_role', // PostgreSQL
	'current_schema', // PostgreSQL
	'current_catalog', // PostgreSQL
	'current_setting', // PostgreSQL (function returning runtime config value)
	'session_timezone', // PostgreSQL
	'local_timezone', // PostgreSQL/Oracle (derived)
	'timezone', // PostgreSQL (sometimes used in SET/AT TIME ZONE)

	// SQL Server specific
	'@@spid',
	'@@trancount',
	'@@error',
	'@@rowcount',
	'@@identity',
	'@@version',
	'@@servername',
	'@@language',
	'@@datefirst',
	'@@dbts',
	'@@nestlevel',
	'@@cpu_busy',
	'@@connections',
	'@@idle',
	'@@pack_received',
	'@@pack_sent',
	'@@packet_errors',
	'@@procid',
	'@@timeticks',
	'@@total_errors',
	'@@total_read',
	'@@total_write',

	// MySQL specific
	'last_insert_id',
	'found_rows',
	'row_count',
	'version', // MySQL/PostgreSQL/SQL Server
	'connection_id', // MySQL
	'current_user()', // Function form
	'database()', // Current DB in MySQL
	'schema()', // Synonym for `database()` in MySQL
	'user()', // Full username@host
	'uuid', // Built-in in some contexts

	// PostgreSQL specific
	'backend_pid',
	'pg_backend_pid',
	'inet_server_addr',
	'inet_server_port',
	'inet_client_addr',
	'inet_client_port',
	'pg_postmaster_start_time',
	'pg_is_in_recovery',
	'pg_current_xlog_location',
	'pg_last_xlog_receive_location',
	'pg_last_xlog_replay_location',
	'pg_current_wal_lsn',
	'pg_is_in_backup',
	'pg_is_other_temp_schema',
	'pg_trigger_depth'
];

const sqlFunctions = [
	// Aggregate functions
	'avg',
	'count',
	'sum',
	'min',
	'max',
	'group_concat', // MySQL, SQLite
	'string_agg', // PostgreSQL
	'json_agg', // PostgreSQL
	'jsonb_agg', // PostgreSQL
	'array_agg', // PostgreSQL

	// Conditional functions
	'coalesce',
	'nullif',
	'case',
	'iif', // SQL Server
	'if', // MySQL

	// String functions
	'length',
	'char_length',
	'octet_length',
	'lower',
	'upper',
	'trim',
	'ltrim',
	'rtrim',
	'substring',
	'substr',
	'concat',
	'concat_ws',
	'replace',
	'position',
	'instr',
	'left',
	'right',
	'repeat',
	'reverse',
	'ascii',
	'chr', // PostgreSQL
	'initcap', // PostgreSQL
	'translate',
	'regexp_replace',
	'regexp_matches',
	'regexp_split_to_array',
	'regexp_split_to_table',

	// Numeric and math functions
	'abs',
	'round',
	'floor',
	'ceil',
	'ceiling',
	'power',
	'sqrt',
	'mod',
	'sign',
	'exp',
	'ln',
	'log',
	'log10',
	'pi',
	'degrees',
	'radians',
	'trunc',
	'width_bucket',
	'random',
	'rand', // MySQL, SQL Server

	// Date and time functions
	'now',
	'current_date',
	'current_time',
	'current_timestamp',
	'localtime',
	'localtimestamp',
	'getdate',
	'getutcdate',
	'curdate',
	'curtime',
	'sysdate',
	'datediff',
	'dateadd',
	'extract',
	'date_part',
	'age',
	'to_char',
	'to_date',
	'to_timestamp',
	'make_date',
	'make_time',
	'make_timestamp',
	'date_trunc',
	'timeofday', // PostgreSQL

	// Logical/utility functions
	'ifnull',
	'isnull',
	'nvl', // Oracle
	'greatest',
	'least',

	// JSON functions
	'json_extract',
	'json_set',
	'json_insert',
	'json_replace',
	'json_remove',
	'json_type',
	'json_valid',
	'json_object',
	'json_array',
	'json_value',
	'json_query',
	'json_merge',
	'jsonb_extract_path', // PostgreSQL
	'jsonb_set',
	'jsonb_insert',
	'jsonb_build_object',
	'jsonb_build_array',
	'jsonb_path_query',
	'jsonb_path_exists',
	'jsonb_pretty',

	// Array functions (PostgreSQL)
	'array',
	'array_agg',
	'unnest',
	'array_append',
	'array_prepend',
	'array_cat',
	'array_remove',
	'array_replace',
	'array_length',
	'array_position',
	'array_to_string',
	'string_to_array',
	'cardinality',

	// Type conversion functions
	'cast',
	'convert',
	'typeof', // SQLite
	'pg_typeof', // PostgreSQL
	'parse', // SQL Server

	// Window functions
	'row_number',
	'rank',
	'dense_rank',
	'ntile',
	'lag',
	'lead',
	'first_value',
	'last_value',
	'nth_value',
	'percent_rank',
	'cume_dist',

	// System and info functions
	'current_user',
	'session_user',
	'user',
	'version',
	'current_setting', // PostgreSQL
	'has_table_privilege', // PostgreSQL
	'pg_backend_pid', // PostgreSQL
	'pg_is_in_recovery',
	'index_list', // Sqlite

	// Network functions (PostgreSQL)
	'inet_ntoa',
	'inet_aton',
	'host',
	'masklen',
	'set_masklen',
	'text',
	'abbrev',
	'family',
	'broadcast',

	// Full-text search functions (PostgreSQL)
	'to_tsvector',
	'to_tsquery',
	'plainto_tsquery',
	'phraseto_tsquery',
	'ts_rank',
	'ts_headline',

	// Geometry/spatial (PostGIS, MySQL spatial)
	'st_distance',
	'st_contains',
	'st_intersects',
	'st_within',
	'st_astext',
	'st_geomfromtext',
	'st_transform',
	'st_buffer'
];

export const sqlLanguage = {
	defaultToken: '',
	ignoreCase: true,
	tokenPostfix: '.sql',

	keywords: sqlKeywords,
	types: sqlTypes,
	constants: sqlConstants,
	functions: sqlFunctions,

	tokenizer: {
		root: [
			// Comments: -- line comment
			[/--.*$/, 'comment'],

			// Comments: /* block comment */
			[/\/\*/, 'comment', '@comment'],

			// Variables (e.g., $VARIABLE_NAME)
			[/\$[a-zA-Z_][a-zA-Z0-9_]*/, 'variable'],

			// Strings
			[/'([^']|'')*'/, 'string'],

			// Function calls like NOW()
			[/\b(now|current_timestamp|sysdate|getdate)\s*\(\)/, 'constant'],

			// Identifiers in double quotes
			[/"([^"]|"")*"/, 'identifier'],

			// Brackets and operators
			[/[()[\]]/, '@brackets'],
			[/[;,.]/, 'delimiter'],
			[/[<>=!%&+\-*/|~^]/, 'operator'],

			// Keywords (tokenPostfix adds .sql automatically)
			[
				/\b\w+\b/,
				{
					cases: {
						'@keywords': 'keyword',
						'@types': 'type',
						'@constants': 'constant',
						'@functions': 'function',
						'@default': 'identifier'
					}
				}
			]
		],

		comment: [
			[/[^/*]+/, 'comment'],
			[/\*\//, 'comment', '@pop'],
			[/[/*]/, 'comment']
		]
	}
};
