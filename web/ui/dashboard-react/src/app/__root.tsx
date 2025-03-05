import { lazy, Suspense } from 'react';
import { createRootRouteWithContext, Outlet } from '@tanstack/react-router';
import { isProductionMode } from '@/lib/env';

import type { AuthContext } from '@/hooks/use-auth';

const TanStackRouterDevTools = isProductionMode
	? () => null
	: lazy(() =>
			import('@tanstack/router-devtools').then(res => ({
				default: res.TanStackRouterDevtools,
			})),
		);

type RouterContext = {
	auth: AuthContext | null
};

export const Route = createRootRouteWithContext<RouterContext>()({
	component: () => (
		<>
			<Outlet />
			<Suspense>{/* <TanStackRouterDevTools /> */}</Suspense>
		</>
	),
});
