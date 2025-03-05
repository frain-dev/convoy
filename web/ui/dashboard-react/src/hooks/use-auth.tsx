import { CONVOY_AUTH_TOKENS_KEY, CONVOY_AUTH_KEY } from '@/lib/constants';

import type { CachedAuth } from '@/models/auth.model';

type AuthDetailsTokenJson = {
	access_token: string;
	refresh_token: string;
};

export function useAuth() {
	function getCachedAuthTokens() {
		const authDetails = localStorage.getItem(CONVOY_AUTH_TOKENS_KEY);

		if (authDetails && authDetails !== 'undefined') {
			const token = JSON.parse(authDetails) as AuthDetailsTokenJson;

			return {
				access_token: token.access_token,
				refresh_token: token.refresh_token,
				isLoggedIn: true,
			};
		}

		return { isLoggedIn: false };
	}

	function getCachedAuthProfile(): null | CachedAuth {
		const authProfile = localStorage.getItem(CONVOY_AUTH_KEY);

		if (authProfile && authProfile !== 'undefined')
			return JSON.parse(authProfile);

		return null;
	}

	return {
		getTokens: getCachedAuthTokens,
		getCurrentUser: getCachedAuthProfile,
	};
}

export type AuthContext = ReturnType<typeof useAuth>;
