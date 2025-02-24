import { lazy, Suspense } from 'react';
import { createRootRoute, Outlet } from '@tanstack/react-router';
import { isProductionMode } from '@/lib/env';

const TanStackRouterDevTools = isProductionMode
	? () => null
	: lazy(() =>
			import('@tanstack/router-devtools').then(res => ({
				default: res.TanStackRouterDevtools,
			})),
		);

export const Route = createRootRoute({
	component: () => (
		<>
			<Outlet />
			<Suspense>
				<TanStackRouterDevTools />
			</Suspense>
		</>
	),
});
