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
		// `async ({}, use)` is how playwright declares a fixture that depends on no
		// other fixture -- it reads the destructuring pattern to find them, so the
		// empty one is the declaration, not an oversight.
		files: ['tests/**/*.ts'],
		rules: {
			'no-empty-pattern': ['error', { allowObjectPatternsAsParameters: true }]
		}
	},
	{
		// The app may not import the test suite. Only the specs and the screenshot
		// specs may, and both sit under src/ so they can live beside what they
		// cover -- which is what makes this worth enforcing: from the file tree, a
		// spec and a component look the same, and one stray import would put
		// @playwright/test in the shipped bundle.
		files: ['src/**/*.ts', 'src/**/*.svelte'],
		ignores: ['src/**/*.shot.ts', 'src/**/*.spec.ts'],
		rules: {
			'no-restricted-imports': [
				'error',
				{
					patterns: [
						{ group: ['**/tests/e2e/*'], message: 'Test-only; import it from a *.shot.ts.' }
					]
				}
			]
		}
	},
	{
		ignores: [
			'**/*.svelte.ts',
			// Wails-generated bindings; do not lint (namespaces, `any`, etc.)
			'src/lib/bindings/**'
		]
	}
);
