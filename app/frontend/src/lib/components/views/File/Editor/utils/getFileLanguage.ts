/**
 * Determines the Monaco editor language ID based on file extension
 */
export function getFileLanguage(filename: string): string {
	const ext = filename.split('.').pop()?.toLowerCase();
	const baseName = filename.split('/').pop()?.toLowerCase();

	switch (ext) {
		case 'sql':
			return 'sql-custom';
		case 'py':
			return 'python';
		case 'js':
			return 'javascript';
		case 'ts':
			return 'typescript';
		case 'json':
			return 'json';
		case 'md':
			return 'markdown';
		case 'yaml':
		case 'yml':
			return 'yaml';
		case 'xml':
			return 'xml';
		case 'html':
			return 'html';
		case 'css':
			return 'css';
		case 'sh':
			return 'shell';
		default:
			if (baseName === '.env') return 'dotenv';
			if (baseName === '.theme') return 'css';
			if (baseName === '.config') return 'json';
			if (baseName === '.lint') return 'json';
			return 'plaintext';
	}
}
