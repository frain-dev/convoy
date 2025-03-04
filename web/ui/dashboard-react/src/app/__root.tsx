import { lazy, Suspense } from 'react';
import { createRootRouteWithContext, Outlet } from '@tanstack/react-router';
import { isProductionMode } from '@/lib/env';

const TanStackRouterDevTools = isProductionMode
	? () => null
	: lazy(() =>
			import('@tanstack/router-devtools').then(res => ({
				default: res.TanStackRouterDevtools,
			})),
		);

type RouterContext = {};

export const Route = createRootRouteWithContext<RouterContext>()({
	component: () => (
		<>
			<Outlet />
			<Suspense>{/* <TanStackRouterDevTools /> */}</Suspense>
		</>
	),
});
