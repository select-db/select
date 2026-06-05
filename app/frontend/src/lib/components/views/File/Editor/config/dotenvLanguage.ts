export const dotenvLanguage = {
	defaultToken: '',
	tokenPostfix: '.env',

	tokenizer: {
		root: [
			// Comments (lines starting with #)
			[/^\s*#.*$/, 'comment'],

			// Key part (variable name before =)
			[/^[A-Z_][A-Z0-9_]*(?=\s*=)/, 'variable.name'],

			// Equals sign
			[/=/, 'delimiter'],

			// Quoted strings (double quotes)
			[/"([^"\\]|\\.)*"/, 'string'],

			// Quoted strings (single quotes)
			[/'([^'\\]|\\.)*'/, 'string'],

			// Unquoted values
			[/[^\s]+/, 'string']
		]
	}
};
