import { ensureCanAccessPrivatePages } from '@/lib/auth';
import { Outlet, createFileRoute } from '@tanstack/react-router';

import { DashboardLayout } from '@/components/dashboard';

export const Route = createFileRoute('/projects/_layout')({
	beforeLoad({ context }) {
		ensureCanAccessPrivatePages(context.auth?.getTokens().isLoggedIn);
	},
	component: ProjectsLayout,
});

function ProjectsLayout() {
	return (
		<DashboardLayout>
			<Outlet />
		</DashboardLayout>
	);
}
