import js from '@eslint/js';
import globals from 'globals';
import reactHooks from 'eslint-plugin-react-hooks';
import reactRefresh from 'eslint-plugin-react-refresh';
import tseslint from 'typescript-eslint';
import react from 'eslint-plugin-react';

export default tseslint.config(
	{ ignores: ['dist'] },
	{
		settings: { react: { version: '18.3' } },
		extends: [js.configs.recommended, ...tseslint.configs.strictTypeChecked],
		files: ['**/*.{ts,tsx}'],
		languageOptions: {
			ecmaVersion: 'latest',
			globals: globals.browser,
			parserOptions: {
				project: ['./tsconfig.node.json', './tsconfig.app.json'],
				tsconfigRootDir: import.meta.dirname
			}
		},
		plugins: {
			'react-hooks': reactHooks,
			'react-refresh': reactRefresh,
			react
		},
		rules: {
			...reactHooks.configs.recommended.rules,
			'react-refresh/only-export-components': ['warn', { allowConstantExport: true }],
			...react.configs.recommended.rules,
			...react.configs['jsx-runtime'].rules
		}
	}
);
