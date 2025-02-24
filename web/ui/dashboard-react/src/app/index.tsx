import { createFileRoute } from '@tanstack/react-router';
import { router } from '@/lib/router';
import { ensureCanAccessPrivatePages } from '@/lib/auth';

export const Route = createFileRoute('/')({
	beforeLoad() {
		ensureCanAccessPrivatePages();
		router.navigate({
			to: '/projects',
			// @ts-expect-error `pathname` is a defined route
			from: router.state.location.pathname,
		});
	},
	component: Index,
});

function Index() {
	return <div></div>;
}
