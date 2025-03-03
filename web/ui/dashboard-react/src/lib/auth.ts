import { redirect } from '@tanstack/react-router';
import { router } from '@/lib/router';
import { getCachedAuthTokens } from '@/services/auth.service';

export function ensureCanAccessPrivatePages() {
	const { authState } = getCachedAuthTokens();

	if (authState == false) {
		redirect({
			// @ts-expect-error `pathname` is a defined route
			from: router.state.location.pathname,
			to: '/login',
			throw: true,
		});
	}

	return true;
}
