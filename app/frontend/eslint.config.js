import prettier from 'eslint-config-prettier';
import js from '@eslint/js';
import { includeIgnoreFile } from '@eslint/compat';
import svelte from 'eslint-plugin-svelte';
import globals from 'globals';
import { fileURLToPath } from 'node:url';
import ts from 'typescript-eslint';
const gitignorePath = fileURLToPath(new URL('./.gitignore', import.meta.url));

export default ts.config(
	includeIgnoreFile(gitignorePath),
	js.configs.recommended,
	...ts.configs.recommended,
	...svelte.configs['flat/recommended'],
	prettier,
	...svelte.configs['flat/prettier'],
	{
		languageOptions: {
			globals: {
				...globals.browser,
				...globals.node
			}
		}
	},
	{
		files: ['**/*.svelte'],
		languageOptions: {
			parserOptions: {
				parser: ts.parser
			}
		},
		rules: {
			'svelte/no-useless-children-snippet': 'off',
			'jsx-a11y/aria-label': 'off',
			'svelte/a11y-no-static-element-interactions': 'off', // Fixed rule name
			'svelte/a11y-click-events-have-key-events': 'off' // Fixed rule name
		}
	},
	{
		ignores: [
			'**/*.svelte.ts',
			// Wails-generated bindings; do not lint (namespaces, `any`, etc.)
			'src/lib/wailsjs/**'
		]
	}
);
