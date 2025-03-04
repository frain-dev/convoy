import { createFileRoute } from '@tanstack/react-router';
import { ensureCanAccessPrivatePages } from '@/lib/auth';
import { DashboardLayout } from '@/components/dashboard';

export const Route = createFileRoute('/projects/')({
	beforeLoad() {
		ensureCanAccessPrivatePages();
	},
	component: RouteComponent,
});

function RouteComponent() {
	return (
		<DashboardLayout>
			<div className='text-lg'>Lorem ipsum dolor sit amet consectetur, adipisicing elit. Dicta, blanditiis placeat repellendus, corrupti numquam accusantium ipsam culpa voluptas consectetur dolor obcaecati. Eius laboriosam eligendi necessitatibus veniam et, nihil consequuntur magnam?</div>
		</DashboardLayout>
	);
}
