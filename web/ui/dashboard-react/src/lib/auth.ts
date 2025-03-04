import { redirect } from '@tanstack/react-router';

export function ensureCanAccessPrivatePages(isLoggedIn?: boolean) {
	if (isLoggedIn) return true;
	throw redirect({ to: '/login' });
}
