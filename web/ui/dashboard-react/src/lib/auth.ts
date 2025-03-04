import { redirect } from '@tanstack/react-router';
import { getCachedAuthTokens } from '@/services/auth.service';

export function ensureCanAccessPrivatePages() {
	const { authState } = getCachedAuthTokens();

	if (authState == false) {
		redirect({ to: '/login', throw: true });
	}

	return true;
}
