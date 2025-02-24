import { createFileRoute } from '@tanstack/react-router';
import { ensureCanAccessPrivatePages } from '@/lib/auth';

export const Route = createFileRoute('/projects/')({
	beforeLoad() {
		ensureCanAccessPrivatePages();
	},
	component: RouteComponent,
});

function RouteComponent() {
	return <h2 className="text-4xl font-semibold p-4">Projects</h2>;
}
