import { createFileRoute, Link } from '@tanstack/react-router';

import { Button } from '@/components/ui/button';
import { DashboardLayout } from '@/components/dashboard';

import { ensureCanAccessPrivatePages } from '@/lib/auth';
import { getUserPermissions } from '@/services/auth.service';

import sourcesEmptyState from "../../../../../assets/img/sources-empty-state.png"

export const Route = createFileRoute('/projects_/$projectId/sources/')({
	component: RouteComponent,
	beforeLoad({ context }) {
		ensureCanAccessPrivatePages(context.auth?.getTokens().isLoggedIn);
	},
	loader: async () => {
		const perms = await getUserPermissions();

		return {
			canManageSources: perms.includes('Sources|MANAGE'),
			sources: { content: [] },
		};
	},
});

function RouteComponent() {
	const { canManageSources, sources } = Route.useLoaderData();
	const { projectId } = Route.useParams();

	if (sources.content.length === 0) {
		return (
			<DashboardLayout showSidebar={true}>
				<div className="m-auto">
					<div className="flex flex-col items-center justify-center">
						<img
							src={sourcesEmptyState}
							alt="No subscriptions created"
							className="h-40 mb-6"
						/>
						<h2 className="font-bold mb-4 text-base text-neutral-12 text-center">
							Create your first source
						</h2>

						<p className="text-neutral-10 text-sm mb-6 max-w-[410px] text-center">
							Sources are how your webhook events are routed into the Convoy.
						</p>

						<Button
							className="mt-9 mb-9 hover:bg-new.primary-400 bg-new.primary-400 text-white-100 hover:text-white-100 px-5 py-3 text-xs"
							disabled={!canManageSources}
							asChild
						>
							<Link
								to="/projects/$projectId/sources/new"
								params={{ projectId }}
							>
								<svg
									width="22"
									height="22"
									className="scale-100"
									fill="#ffffff"
								>
									<use xlinkHref="#plus-icon"></use>
								</svg>
								Connect a source
							</Link>
						</Button>
					</div>
				</div>
			</DashboardLayout>
		);
	}

	return (
		<DashboardLayout showSidebar={true}>
			<div>List sources</div>
		</DashboardLayout>
	);
}
