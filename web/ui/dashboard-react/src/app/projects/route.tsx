import { ensureCanAccessPrivatePages } from '@/lib/auth';
import { Outlet, createFileRoute } from '@tanstack/react-router';

import { DashboardLayout } from '@/components/dashboard';

export const Route = createFileRoute('/projects')({
	beforeLoad({ context }) {
		ensureCanAccessPrivatePages(context.auth?.getTokens().isLoggedIn);
	},
	// TODO use Zustand instead because this provider here will cause the whole nodes to rerender and that's not performant
	component: ProjectsLayout,
});

function ProjectsLayout() {
	return (
		<DashboardLayout showSidebar={true}>
			<Outlet />
		</DashboardLayout>
	);
}
