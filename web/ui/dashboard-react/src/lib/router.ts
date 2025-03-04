import { createRouter } from '@tanstack/react-router';
import { routeTree } from '../routes.gen';

export const router = createRouter({ routeTree, context: { auth: null } });

declare module '@tanstack/react-router' {
	interface Register {
		router: typeof router;
	}
}
