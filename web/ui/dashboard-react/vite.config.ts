import path from 'node:path';
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { TanStackRouterVite } from '@tanstack/router-plugin/vite';

// https://vite.dev/config/
export default defineConfig({
	plugins: [
		TanStackRouterVite({
			autoCodeSplitting: true,
			routesDirectory: './src/app',
			generatedRouteTree: './src/routes.gen.ts',
		}),
		react(),
	],
	resolve: {
		alias: {
			'@': path.resolve(__dirname, './src'),
		},
	},
});
