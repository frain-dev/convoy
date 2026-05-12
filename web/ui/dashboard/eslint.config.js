// @ts-check
const eslint = require('@eslint/js');
const { defineConfig } = require('eslint/config');
const tseslint = require('typescript-eslint');
const angular = require('angular-eslint');

/**
 * Pragmatic baseline for an existing Angular app: catch real foot-guns without
 * forcing a full inject() migration, stylistic churn, or strict `any` cleanup in one pass.
 */
module.exports = defineConfig([
	{
		ignores: ['dist/**', 'node_modules/**', 'coverage/**', 'source-maps/**', '.angular/**']
	},
	{
		files: ['**/*.ts'],
		extends: [
			eslint.configs.recommended,
			...tseslint.configs.recommended,
			...angular.configs.tsRecommended
		],
		processor: angular.processInlineTemplates,
		languageOptions: {
			parserOptions: {
				projectService: true,
				tsconfigRootDir: __dirname
			}
		},
		rules: {
			// Migration-only; enable when you run the inject() schematic across the repo.
			'@angular-eslint/prefer-inject': 'off',
			'@angular-eslint/prefer-standalone': 'off',
			'@angular-eslint/no-input-rename': 'off',
			'@angular-eslint/no-output-rename': 'off',
			'@angular-eslint/no-output-on-prefix': 'off',
			'@angular-eslint/no-output-native': 'off',
			'@angular-eslint/no-empty-lifecycle-method': 'off',
			'@angular-eslint/use-lifecycle-interface': 'off',

			// Mixed `app-*` and `convoy-*` selectors today; tighten later with a single prefix policy.
			'@angular-eslint/component-selector': 'off',
			'@angular-eslint/directive-selector': 'off',

			'@typescript-eslint/no-wrapper-object-types': 'off',
			'@typescript-eslint/no-empty-object-type': 'off',

			'@typescript-eslint/no-explicit-any': 'off',
			'@typescript-eslint/no-inferrable-types': 'off',
			'@typescript-eslint/consistent-indexed-object-style': 'off',
			'@typescript-eslint/no-unused-expressions': 'off',

			'@typescript-eslint/no-unused-vars': [
				'error',
				{
					argsIgnorePattern: '^_',
					varsIgnorePattern: '^_',
					caughtErrors: 'none'
				}
			],
			// Many lifecycle hooks are intentionally empty until wired up.
			'@typescript-eslint/no-empty-function': 'off',
			// ESLint core (eslint:recommended); most of this codebase predates systematic `cause` chaining.
			'preserve-caught-error': 'off',

			'no-async-promise-executor': 'off',
			'no-var': 'off',
			'no-useless-escape': 'off',
			'no-useless-catch': 'off',
			'no-useless-assignment': 'off',

			'no-empty': ['error', { allowEmptyCatch: true }],
			'prefer-const': 'off'
		}
	},
	{
		files: ['**/*.html'],
		extends: [angular.configs.templateRecommended],
		rules: {
			// Add `angular.configs.templateAccessibility` when you want stricter a11y gates in CI.
			// Large legacy template surface: enable `===` / `@if` migration in follow-up PRs.
			'@angular-eslint/template/eqeqeq': 'off',
			'@angular-eslint/template/prefer-control-flow': 'off'
		}
	}
]);
