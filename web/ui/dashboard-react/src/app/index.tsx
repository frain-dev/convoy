import { createFileRoute } from '@tanstack/react-router';
import { router } from '@/lib/router';
import { ensureCanAccessPrivatePages } from '@/lib/auth';

export const Route = createFileRoute('/')({
	beforeLoad() {
		ensureCanAccessPrivatePages();
		router.navigate({ to: '/projects' });
	},
	component: Index,
});

function Index() {
	return <div></div>;
}
