import { createFileRoute } from '@tanstack/react-router';
import { ensureCanAccessPrivatePages } from '@/lib/auth';
import { Dashboard } from '@/components/layout/dashboard';

export const Route = createFileRoute('/projects/')({
	beforeLoad() {
		ensureCanAccessPrivatePages();
	},
	component: RouteComponent,
});

function RouteComponent() {
	return (
		<Dashboard>
			{/* <div>Page</div> */}
		</Dashboard>
	);
}
