import { createFileRoute, Link } from '@tanstack/react-router';

import { Button } from '@/components/ui/button';
import { DashboardLayout } from '@/components/dashboard';

import { ensureCanAccessPrivatePages } from '@/lib/auth';
import { getUserPermissions } from '@/services/auth.service';
import * as subscriptionsService from '@/services/subscriptions.service';

import subscriptionsEmptyStateImg from '../../../../../assets/img/subscriptions-empty-state.png';

export const Route = createFileRoute('/projects_/$projectId/subscriptions/')({
	component: ListSubcriptionsPage,
	beforeLoad({ context }) {
		ensureCanAccessPrivatePages(context.auth?.getTokens().isLoggedIn);
	},
	loader: async ({ params }) => {
		const perms = await getUserPermissions();
    // TODO: extract pagination params and endpointId and name from params
		const subscriptions = await subscriptionsService.getSubscriptions({
			endpointId: params.projectId,
		});

		return {
			canManageSubscriptions: perms.includes('Subscriptions|MANAGE'),
			subscriptions,
		};
	},
});

function ListSubcriptionsPage() {
	const { canManageSubscriptions, subscriptions } = Route.useLoaderData();
	const { projectId } = Route.useParams();

	if (subscriptions.content.length === 0) {
		return (
			<DashboardLayout showSidebar={true}>
				<div className="m-auto">
					<div className="flex flex-col items-center justify-center">
						<img
							src={subscriptionsEmptyStateImg}
							alt="No subscriptions created"
							className="h-40 mb-6"
						/>
						<h2 className="font-bold mb-4 text-base text-neutral-12 text-center">
							You currently do not have any subscriptions
						</h2>

						<p className="text-neutral-10 text-sm mb-6 max-w-[410px] text-center">
							Webhook subscriptions lets you define the source of your webhook
							and the destination where any webhook event should be sent. It is
							what allows Convoy to identify and proxy your webhooks.
						</p>

						<Button
							className="mt-9 mb-9 hover:bg-new.primary-400 bg-new.primary-400 text-white-100 hover:text-white-100 px-5 py-3 text-xs"
							disabled={!canManageSubscriptions}
							asChild
						>
							<Link
								to="/projects/$projectId/subscriptions/new"
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
								Create a subscription
							</Link>
						</Button>
					</div>
				</div>
			</DashboardLayout>
		);
	}

	return (
		<DashboardLayout showSidebar={true}>
			<div className="m-auto">LIst Subscriptions</div>
		</DashboardLayout>
	);
}
