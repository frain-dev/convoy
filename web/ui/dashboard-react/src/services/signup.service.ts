import { request } from './http.service';

import type { HttpResponse } from '@/models/global.model';

type SignUpParams = {
	email: string;
	password: string;
	first_name: string;
	last_name: string;
	org_name: string;
};

export async function signUp(
	requestDetails: SignUpParams,
	deps: { httpReq: typeof request } = { httpReq: request },
) {
	const res = await deps.httpReq({
		url: '/auth/register',
		body: requestDetails,
		method: 'post',
	});

	return res;
}

export async function getSignUpConfig(
	deps: { httpReq: typeof request } = { httpReq: request },
) {
	const res = await deps.httpReq<boolean>({
		url: '/configuration/is_signup_enabled',
		method: 'get',
	});

	return res;
}

export async function signUpWithSAML(
	deps: { httpReq: typeof request } = { httpReq: request },
) {
	const res = await deps.httpReq({ url: '/auth/sso', method: 'get' });

	return res;
}
