import { redirect } from '@tanstack/react-router';
import { router } from '@/lib/router';
import { authDetails } from '@/services/http.service';

export function ensureCanAccessPrivatePages() {
	const { authState } = authDetails();

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
