import { createFileRoute } from '@tanstack/react-router';

import * as subscriptionsService from '@/services/subscriptions.service';

export const Route = createFileRoute(
	'/projects_/$projectId/subscriptions/$subscriptionId',
)({
	component: RouteComponent,
	async loader({ params }) {
		const subscription = await subscriptionsService.getSubscription(
			params.subscriptionId,
		);

		return { subscription };
	},
});

function RouteComponent() {
	const { subscription } = Route.useLoaderData();
	return <div>Update Subscription: {subscription.name}</div>;
}
