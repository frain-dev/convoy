import { request } from './http.service';
import {
	CONVOY_AUTH_KEY,
	CONVOY_AUTH_TOKENS_KEY,
	CONVOY_LAST_AUTH_LOCATION_KEY,
} from '@/lib/constants';
import { router } from '@/lib/router';

import type { CachedAuth } from '@/models/auth.model';

type AuthDetailsTokenJson = {
	access_token: string;
	refresh_token: string;
};

export function getCachedAuthTokens() {
	const authDetails = localStorage.getItem(CONVOY_AUTH_TOKENS_KEY);

	if (authDetails && authDetails !== 'undefined') {
		const token = JSON.parse(authDetails) as AuthDetailsTokenJson;

		return {
			access_token: token.access_token,
			refresh_token: token.refresh_token,
			authState: true,
		};
	}

	return { authState: false };
}

export function getCachedAuthProfile(): null | CachedAuth {
	const authProfile = localStorage.getItem(CONVOY_AUTH_KEY);

	if (authProfile && authProfile !== 'undefined')
		return JSON.parse(authProfile);

	return null;
}

type LoginRequestDetails = {
	email: string;
	password: string;
};

type LoginDependencies = {
	httpReq: typeof request;
};

export async function login(
	requestDetails: LoginRequestDetails,
	deps: LoginDependencies = { httpReq: request },
) {
	const { email: username, password } = requestDetails;

	const res = await deps.httpReq<CachedAuth>({
		url: '/auth/login',
		body: { username, password },
		method: 'post',
	});

	localStorage.setItem(CONVOY_AUTH_KEY, JSON.stringify(res.data));
	localStorage.setItem(CONVOY_AUTH_TOKENS_KEY, JSON.stringify(res.data.token));

	return;
}

export async function loginWithSAML(
	deps: LoginDependencies = { httpReq: request },
) {
	const res = await deps.httpReq<{ redirectUrl: string }>({
		url: '/auth/sso',
		method: 'get',
	});

	return res;
}

export function logUserOut() {
	// save previous location before session timeout
	if (!router.state.location.pathname.startsWith('/login')) {
		localStorage.removeItem(CONVOY_AUTH_TOKENS_KEY);
		localStorage.setItem(CONVOY_LAST_AUTH_LOCATION_KEY, location.href);
	}

	// then move user to login page
	router.navigate({ to: '/' });
}
