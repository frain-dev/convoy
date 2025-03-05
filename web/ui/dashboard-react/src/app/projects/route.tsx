import { ensureCanAccessPrivatePages } from '@/lib/auth';
import { Outlet, createFileRoute } from '@tanstack/react-router';

import { DashboardLayout } from '@/components/dashboard';
import { OrganisationProvider } from '@/contexts/organisation';

export const Route = createFileRoute('/projects')({
	beforeLoad({ context }) {
		ensureCanAccessPrivatePages(context.auth?.getTokens().isLoggedIn);
	},
	component: ProjectsLayout,
});

function ProjectsLayout() {
	return (
		// TODO use Zustand instead because this provider here will cause the whole nodes to rerender and that's not performant
		<OrganisationProvider>
			<DashboardLayout>
				<Outlet />
			</DashboardLayout>
		</OrganisationProvider>
	);
}
