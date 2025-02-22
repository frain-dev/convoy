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

export async function getSignUpConfig(): Promise<HttpResponse<boolean>> {
	// deps: { httpReq: typeof request } = { httpReq: request },
	// const res = await deps.httpReq({
	// 	url: '/configuration/is_sign_up_enabled',
	// 	method: 'get',
	// });

	return { data: true, message: '', status: true };
}

export async function signUpWithSAML(
	deps: { httpReq: typeof request } = { httpReq: request },
) {
	const res = await deps.httpReq({ url: '/auth/sso', method: 'get' });

	return res;
}
