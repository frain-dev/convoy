import { createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/signup')({
	component: SignUpPage,
});

function SignUpPage() {
	return <h2 className="text-4xl font-semibold p-4">Sign Up</h2>;
}
