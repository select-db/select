import type { Icons } from '$lib/system/Icon/types';

/** Metadata shape for DB column nodes (subset used for icon choice). */
export type ColumnMetadata = { isPrimaryKey?: boolean };

export interface IconConfig {
	icon: Icons;
	spacer: boolean;
	size?: number;
}

/**
 * O(1) exact-match lookup for all simple type → icon mappings.
 * Column prefix types and the primary-key override are handled separately below.
 */
const EXACT_ICON_MAP = new Map<string, IconConfig>([
	['db_instance', { icon: 'db', spacer: false }],
	['schema', { icon: 'schema', spacer: false }],
	['table', { icon: 'table', spacer: false }],
	['view', { icon: 'table', spacer: false }],
	['index', { icon: 'index', spacer: true }],
	['trigger', { icon: 'bolt', spacer: false }],
	['column:integer', { icon: 'number', spacer: true }],
	['column:jsonb', { icon: 'json', spacer: true }],
	['column:json', { icon: 'json', spacer: true }],
	['column:boolean', { icon: 'checkbox', spacer: true }],
	['quick_action:query', { icon: 'chevron-right', spacer: false, size: 19 }],
	['quick_action:schema', { icon: 'chevron-right', spacer: false, size: 19 }],
	['quick_action:chat', { icon: 'chevron-right', spacer: false, size: 19 }],
	['quick_action:terminal', { icon: 'chevron-right', spacer: false, size: 19 }],
	['quick_action:settings', { icon: 'chevron-right', spacer: false, size: 19 }],
	['settings', { icon: 'cog', spacer: false, size: 19 }],
	['chat', { icon: 'chat', spacer: false }],
	['terminal', { icon: 'terminal', spacer: false }],
	['diff', { icon: 'diff', spacer: false }],
	['type', { icon: 'code-bracket', spacer: false }],
	['function', { icon: 'function', spacer: false }],
	['db_setting', { icon: 'code-bracket', spacer: false }]
]);

/**
 * Returns icon and spacer for a type (and optional metadata). Used by ItemIcon and modal headers.
 */
export function getIconConfig(type?: string): IconConfig | undefined {
	if (!type) return { icon: 'text', spacer: false };
	if (type === 'file') return { icon: 'paragraph', spacer: false };

	// O(1) exact match covers the vast majority of types
	const exact = EXACT_ICON_MAP.get(type);
	if (exact) return exact;

	// Prefix fallbacks for remaining column variants
	if (type.startsWith('column:character') || type.startsWith('column:text'))
		return { icon: 'text', spacer: true };
	if (type.startsWith('column:timestamp') || type.startsWith('column:datetime'))
		return { icon: 'timestamp', spacer: true };
	if (type.startsWith('column:')) return { icon: 'code-bracket', spacer: true };
	if (type.includes('[]')) return { icon: 'brackets', spacer: false };

	return undefined;
}

/**
 * Returns only the icon. Use when you don't need the spacer (e.g. modal header, buttons).
 */
export function getIcon(type?: string): Icons | undefined {
	return getIconConfig(type)?.icon;
}

type FileIconDef = { icon: Icons; size: number; color: string };

const SPECIAL_FILENAMES: Record<string, FileIconDef> = {
	'[edits].sql': { icon: 'db-upload', size: 16, color: 'var(--gray-800)' },
	'.gitignore': { icon: 'git', size: 19, color: 'var(--orange)' },
	'.gitattributes': { icon: 'git', size: 19, color: 'var(--orange)' },
	'.dockerignore': { icon: 'docker', size: 19, color: 'var(--blue)' },
	Dockerfile: { icon: 'docker', size: 19, color: 'var(--blue)' },
	Makefile: { icon: 'cog', size: 19, color: 'var(--orange)' },
	LICENSE: { icon: 'journal', size: 19, color: 'var(--gray-800)' },
	'go.mod': { icon: 'package', size: 19, color: 'var(--blue)' },
	'go.sum': { icon: 'lock', size: 19, color: 'var(--gray-800)' },
	'package.json': { icon: 'npm', size: 19, color: 'var(--red)' },
	'package-lock.json': { icon: 'lock', size: 19, color: 'var(--gray-800)' },
	'Cargo.toml': { icon: 'package', size: 19, color: 'var(--orange)' },
	'Gemfile': { icon: 'lang-ruby', size: 19, color: 'var(--red)' },
	'Gemfile.lock': { icon: 'lock', size: 19, color: 'var(--gray-800)' },
	'requirements.txt': { icon: 'lang-python', size: 19, color: 'var(--yellow)' },
	'pyproject.toml': { icon: 'lang-python', size: 19, color: 'var(--yellow)' },
	'Pipfile': { icon: 'lang-python', size: 19, color: 'var(--yellow)' },
	'pom.xml': { icon: 'lang-java', size: 19, color: 'var(--red)' },
	'build.gradle': { icon: 'lang-groovy', size: 19, color: 'var(--blue)' },
	'Dockerfile.dev': { icon: 'docker', size: 19, color: 'var(--blue)' },
	'docker-compose.yml': { icon: 'docker', size: 19, color: 'var(--blue)' },
	'docker-compose.yaml': { icon: 'docker', size: 19, color: 'var(--blue)' },
	'deno.json': { icon: 'deno', size: 19, color: 'var(--gray-800)' },
	'deno.lock': { icon: 'deno', size: 19, color: 'var(--gray-800)' },
	'.nvmrc': { icon: 'nodejs', size: 19, color: 'var(--green)' },
	'.npmrc': { icon: 'npm', size: 19, color: 'var(--red)' },
	'yarn.lock': { icon: 'yarn', size: 19, color: 'var(--blue)' },
	'pnpm-lock.yaml': { icon: 'npm', size: 19, color: 'var(--orange)' },
	'vite.config.ts': { icon: 'vite', size: 19, color: 'var(--purple)' },
	'vite.config.js': { icon: 'vite', size: 19, color: 'var(--purple)' },
	'svelte.config.js': { icon: 'svelte', size: 19, color: 'var(--orange)' },
	'tailwind.config.js': { icon: 'tailwind', size: 19, color: 'var(--blue)' },
	'tailwind.config.ts': { icon: 'tailwind', size: 19, color: 'var(--blue)' },
	'angular.json': { icon: 'angular', size: 19, color: 'var(--red)' },
	'.gitmodules': { icon: 'git', size: 19, color: 'var(--orange)' },
	'.gitkeep': { icon: 'git', size: 19, color: 'var(--orange)' }
};

/**
 * The look each group of extensions wears, written once per group rather than
 * once per extension. Colour carries as much of the reading as the shape does:
 * a folder of mixed files is scanned by colour first.
 */
const EXTENSION_GROUPS: [FileIconDef, string[]][] = [
	// SELECT's own files
	[{ icon: 'css', size: 20, color: 'var(--orange)' }, ['.theme']],
	[
		{ icon: 'cog', size: 19, color: 'var(--gray-800)' },
		['.config', '.mk', '.cmake', '.bazel', '.bzl']
	],
	[{ icon: 'eslint', size: 20, color: 'var(--purple)' }, ['.lint']],
	[{ icon: 'code-bracket', size: 19, color: 'var(--gray-800)' }, ['.env']],

	// Queries and the databases they run against
	[{ icon: 'sql', size: 20, color: 'var(--red)' }, ['.sql', '.psql', '.ddl', '.dml']],
	[
		{ icon: 'db', size: 19, color: 'var(--green)' },
		['.sqlite', '.sqlite3', '.db', '.duckdb', '.mdb', '.accdb']
	],
	[{ icon: 'schema', size: 19, color: 'var(--green)' }, ['.dbml']],
	[{ icon: 'prisma', size: 19, color: 'var(--gray-800)' }, ['.prisma']],
	[{ icon: 'graphql', size: 19, color: 'var(--purple)' }, ['.graphql', '.gql']],

	// Web
	[{ icon: 'ts', size: 20, color: 'var(--blue)' }, ['.ts', '.mts', '.cts']],
	[{ icon: 'js', size: 20, color: 'var(--yellow)' }, ['.js', '.mjs', '.cjs']],
	[{ icon: 'react', size: 20, color: 'var(--blue)' }, ['.tsx', '.jsx']],
	[{ icon: 'svelte', size: 19, color: 'var(--orange)' }, ['.svelte']],
	[{ icon: 'vue', size: 19, color: 'var(--green)' }, ['.vue']],
	[{ icon: 'astro', size: 19, color: 'var(--purple)' }, ['.astro']],
	[{ icon: 'html', size: 20, color: 'var(--red)' }, ['.html', '.htm', '.xhtml', '.hbs', '.ejs']],
	[{ icon: 'css', size: 20, color: 'var(--blue)' }, ['.css', '.less', '.styl', '.pcss']],
	[{ icon: 'sass', size: 19, color: 'var(--red)' }, ['.scss', '.sass']],

	// Structured text
	[{ icon: 'json', size: 19, color: 'var(--gray-800)' }, ['.json', '.jsonc', '.json5', '.ndjson']],
	[
		{ icon: 'cog', size: 19, color: 'var(--orange)' },
		['.yaml', '.yml', '.toml', '.ini', '.cfg', '.conf', '.properties']
	],
	[
		{ icon: 'code-bracket', size: 19, color: 'var(--orange)' },
		['.xml', '.plist', '.xsd', '.xsl', '.svgz']
	],
	[{ icon: 'lock', size: 19, color: 'var(--gray-800)' }, ['.lock']],

	// Prose
	[{ icon: 'markdown', size: 20, color: 'var(--blue)' }, ['.md', '.mdx', '.markdown', '.mdown']],
	[
		{ icon: 'paragraph', size: 18, color: 'var(--gray-800)' },
		['.txt', '.text', '.log', '.rst', '.adoc', '.asciidoc']
	],
	[{ icon: 'pdf', size: 19, color: 'var(--red)' }, ['.pdf']],
	[
		{ icon: 'journal', size: 19, color: 'var(--blue)' },
		['.doc', '.docx', '.odt', '.rtf', '.pages', '.tex']
	],
	[{ icon: 'chart', size: 19, color: 'var(--orange)' }, ['.ppt', '.pptx', '.odp', '.keynote']],

	// Tabular
	[{ icon: 'csv', size: 20, color: 'var(--gray-800)' }, ['.csv']],
	[
		{ icon: 'table', size: 19, color: 'var(--green)' },
		['.tsv', '.xls', '.xlsx', '.xlsm', '.ods', '.parquet', '.avro', '.arrow', '.feather']
	],

	// Languages, one icon each: a shape a person already knows beats a colour
	// they have to learn, and the palette has nine colours for sixty languages.
	[{ icon: 'lang-go', size: 20, color: 'var(--blue)' }, ['.go', '.templ']],
	[{ icon: 'lang-rust', size: 19, color: 'var(--orange)' }, ['.rs']],
	[{ icon: 'lang-c', size: 19, color: 'var(--blue)' }, ['.c', '.h']],
	[
		{ icon: 'lang-cpp', size: 20, color: 'var(--blue)' },
		['.cpp', '.cc', '.cxx', '.hpp', '.hh', '.hxx', '.ipp']
	],
	[{ icon: 'lang-csharp', size: 20, color: 'var(--purple)' }, ['.cs', '.csx', '.csproj']],
	[{ icon: 'lang-objc', size: 19, color: 'var(--gray-800)' }, ['.m', '.mm']],
	[
		{ icon: 'lang-python', size: 19, color: 'var(--yellow)' },
		['.py', '.pyi', '.pyw', '.pyx', '.ipynb']
	],
	[{ icon: 'lang-java', size: 19, color: 'var(--red)' }, ['.java', '.jsp', '.jav']],
	[{ icon: 'lang-kotlin', size: 19, color: 'var(--purple)' }, ['.kt', '.kts']],
	[{ icon: 'lang-swift', size: 19, color: 'var(--orange)' }, ['.swift']],
	[{ icon: 'lang-dart', size: 19, color: 'var(--blue)' }, ['.dart']],
	[{ icon: 'lang-php', size: 19, color: 'var(--purple)' }, ['.php', '.phtml', '.php5']],
	[{ icon: 'lang-ruby', size: 19, color: 'var(--red)' }, ['.rb', '.erb', '.gemspec', '.rake']],
	[{ icon: 'lang-lua', size: 19, color: 'var(--blue)' }, ['.lua']],
	[{ icon: 'lang-perl', size: 19, color: 'var(--purple)' }, ['.pl', '.pm', '.pod']],
	[{ icon: 'lang-r', size: 19, color: 'var(--blue)' }, ['.r', '.rmd', '.rdata', '.rds']],
	[{ icon: 'lang-julia', size: 19, color: 'var(--purple)' }, ['.jl']],
	[{ icon: 'lang-matlab', size: 20, color: 'var(--orange)' }, ['.mlx', '.mat', '.sas', '.do']],
	[{ icon: 'lang-haskell', size: 19, color: 'var(--purple)' }, ['.hs', '.lhs', '.cabal']],
	[{ icon: 'lang-elixir', size: 19, color: 'var(--purple)' }, ['.ex', '.exs', '.heex', '.eex']],
	[{ icon: 'lang-erlang', size: 19, color: 'var(--red)' }, ['.erl', '.hrl', '.beam']],
	[
		{ icon: 'lang-lisp', size: 20, color: 'var(--green)' },
		['.clj', '.cljs', '.cljc', '.edn', '.el', '.lisp', '.lsp', '.scm', '.rkt']
	],
	[{ icon: 'lang-scala', size: 19, color: 'var(--red)' }, ['.scala', '.sc', '.sbt']],
	[{ icon: 'lang-elm', size: 19, color: 'var(--blue)' }, ['.elm']],
	[{ icon: 'lang-ocaml', size: 19, color: 'var(--orange)' }, ['.ml', '.mli']],
	[{ icon: 'lang-fsharp', size: 19, color: 'var(--purple)' }, ['.fs', '.fsx', '.fsi']],
	[{ icon: 'lang-zig', size: 19, color: 'var(--orange)' }, ['.zig', '.zon']],
	[{ icon: 'lang-nim', size: 19, color: 'var(--yellow)' }, ['.nim', '.nims', '.nimble']],
	[{ icon: 'lang-vb', size: 19, color: 'var(--purple)' }, ['.vb', '.bas', '.vbs']],
	[{ icon: 'lang-groovy', size: 19, color: 'var(--blue)' }, ['.groovy', '.gradle']],
	[{ icon: 'lang-solidity', size: 19, color: 'var(--gray-800)' }, ['.sol', '.vy']],
	[{ icon: 'lang-assembly', size: 19, color: 'var(--gray-800)' }, ['.asm', '.s', '.wat']],

	// Shells and the machines they run on
	[
		{ icon: 'terminal', size: 19, color: 'var(--green)' },
		['.sh', '.bash', '.zsh', '.fish', '.bat', '.cmd', '.awk', '.sed', '.exp']
	],
	[{ icon: 'powershell', size: 19, color: 'var(--blue)' }, ['.ps1', '.psm1', '.psd1']],
	[{ icon: 'terraform', size: 19, color: 'var(--purple)' }, ['.tf', '.tfvars', '.tfstate', '.hcl']],
	[{ icon: 'nix', size: 19, color: 'var(--blue)' }, ['.nix']],
	[{ icon: 'server', size: 19, color: 'var(--purple)' }, ['.bicep', '.pp', '.sls']],
	[{ icon: 'docker', size: 19, color: 'var(--blue)' }, ['.dockerfile', '.containerfile']],

	// Media
	[
		{ icon: 'image', size: 19, color: 'var(--purple)' },
		[
			'.png',
			'.jpg',
			'.jpeg',
			'.gif',
			'.svg',
			'.webp',
			'.bmp',
			'.ico',
			'.tif',
			'.tiff',
			'.avif',
			'.heic',
			'.psd',
			'.ai',
			'.fig',
			'.sketch'
		]
	],
	[
		{ icon: 'video', size: 19, color: 'var(--purple)' },
		['.mp4', '.mov', '.avi', '.mkv', '.webm', '.wmv', '.m4v', '.mpg', '.mpeg']
	],
	[
		{ icon: 'audio', size: 19, color: 'var(--purple)' },
		['.mp3', '.wav', '.flac', '.ogg', '.m4a', '.aac', '.wma', '.opus', '.mid']
	],
	[{ icon: 'font', size: 19, color: 'var(--orange)' }, ['.ttf', '.otf', '.woff', '.woff2', '.eot']],

	// Everything that arrives packed, signed or compiled
	[
		{ icon: 'archive', size: 19, color: 'var(--orange)' },
		['.zip', '.tar', '.gz', '.tgz', '.bz2', '.xz', '.7z', '.rar', '.zst', '.lz4', '.iso']
	],
	[
		{ icon: 'key', size: 19, color: 'var(--yellow)' },
		['.pem', '.key', '.crt', '.cer', '.pub', '.asc', '.gpg', '.p12', '.pfx', '.jks', '.keystore']
	],
	[
		{ icon: 'binary', size: 19, color: 'var(--gray-800)' },
		['.exe', '.dll', '.so', '.dylib', '.bin', '.wasm', '.o', '.a', '.class', '.pyc', '.dat', '.pak']
	],
	[
		{ icon: 'package', size: 19, color: 'var(--gray-800)' },
		['.jar', '.war', '.deb', '.rpm', '.dmg', '.pkg', '.apk', '.appimage', '.msi', '.whl', '.egg']
	],
	[{ icon: 'diff', size: 19, color: 'var(--green)' }, ['.patch', '.diff']]
];

const EXTENSION_ICONS: Record<string, FileIconDef> = Object.fromEntries(
	EXTENSION_GROUPS.flatMap(([look, extensions]) => extensions.map((ext) => [ext, look]))
);

/** Lowercased: an extension is the same one however it was typed. */
function getFileExtension(fileName: string): string {
	const lastDot = fileName.lastIndexOf('.');
	return lastDot !== -1 ? fileName.slice(lastDot).toLowerCase() : '';
}

function getFileIconConfig(fileName: string): FileIconDef {
	const specialMatch = SPECIAL_FILENAMES[fileName];
	if (specialMatch) return specialMatch;

	const ext = getFileExtension(fileName);
	const extMatch = EXTENSION_ICONS[ext];
	if (extMatch) return extMatch;

	return { icon: 'paragraph', size: 18, color: 'var(--gray-800)' };
}

/** Icon for file nodes (e.g. modal). For size/color, use getItemIconSlot file slot. */
export function getFileIcon(fileName: string): Icons {
	return getFileIconConfig(fileName).icon;
}

/** Content-only slot (no expand icon). */
export type ItemIconContentSlot =
	| { kind: 'file'; icon: Icons; size: number; color: string }
	| { kind: 'database-indicator'; id: string }
	| { kind: 'icon'; icon: Icons; spacer: boolean; size?: number };

/** Container types that show only a folder icon (no content icon) in the tree. */
const FOLDER_ONLY_TYPES = new Set([
	'folder',
	'tables',
	'views',
	'triggers',
	'indexes',
	'columns',
	'types',
	'functions',
	'db_settings'
]);

/** Icon for expand/collapse: folder (collapsed) or folder-open (expanded). */
export const EXPAND_ICON_COLLAPSED: Icons = 'folder';
export const EXPAND_ICON_EXPANDED: Icons = 'folder-open';

/** Full slot: expandable (folder icon + content), folder-only, or content-only. */
export type ItemIconSlot =
	| { kind: 'expandable'; expandIcon: Icons; content: ItemIconContentSlot }
	| { kind: 'folder-only'; expandIcon: Icons }
	| ItemIconContentSlot;

export interface ItemIconSlotInput {
	type: string;
	name?: string;
	id?: string;
	metadata?: Record<string, unknown>;
}

export interface ItemIconSlotOptions {
	noDepth: boolean;
	expanded: boolean;
	expandable: boolean;
	muted: boolean;
}

/** Compute the content slot (icon / database-indicator / file) without expand icon logic. */
function getItemIconContentSlot(
	item: ItemIconSlotInput,
	noDepth: boolean
): ItemIconContentSlot | undefined {
	if (item.type === 'file' && item.name != null) {
		const fileConfig = getFileIconConfig(item.name);
		return { kind: 'file', icon: fileConfig.icon, size: fileConfig.size, color: fileConfig.color };
	}

	if (item.type === 'db_instance' && item.id != null) {
		return { kind: 'database-indicator', id: item.id };
	}

	const config = getIconConfig(item.type);
	if (!config) return undefined;

	return {
		kind: 'icon',
		icon: config.icon,
		spacer: noDepth ? false : config.spacer,
		size: config.size
	};
}

/**
 * Determines what ItemIcon should render for the given item and expansion state.
 * When expandable/expanded and !noDepth, returns expandable (folder icon + content) or folder-only for container types.
 * Returns undefined if no icon should be rendered.
 */
export function getItemIconSlot(
	item: ItemIconSlotInput,
	options: ItemIconSlotOptions
): ItemIconSlot | undefined {
	const { noDepth, expanded, expandable } = options;

	if (!noDepth && expanded) {
		if (FOLDER_ONLY_TYPES.has(item.type ?? '')) {
			return { kind: 'folder-only', expandIcon: EXPAND_ICON_EXPANDED };
		}

		const content = getItemIconContentSlot(item, noDepth);
		if (!content) return undefined;

		return { kind: 'expandable', expandIcon: EXPAND_ICON_EXPANDED, content };
	}
	if (!noDepth && expandable) {
		if (FOLDER_ONLY_TYPES.has(item.type ?? '')) {
			return { kind: 'folder-only', expandIcon: EXPAND_ICON_COLLAPSED };
		}

		const content = getItemIconContentSlot(item, noDepth);
		if (!content) return undefined;

		return { kind: 'expandable', expandIcon: EXPAND_ICON_COLLAPSED, content };
	}

	return getItemIconContentSlot(item, noDepth);
}
