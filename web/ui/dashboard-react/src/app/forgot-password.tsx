import { createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/forgot-password')({
	component: RouteComponent,
});

function RouteComponent() {
	return <h2 className="text-3xl font-semibold p-4">Forgot Password</h2>;
}
