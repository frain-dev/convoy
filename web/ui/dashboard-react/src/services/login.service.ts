import { CONVOY_AUTH_KEY, CONVOY_AUTH_TOKENS_KEY } from '@/lib/constants';
import { request } from './http.service';

import type { CachedAuth } from '@/models/auth.model';

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
