import { request } from './http.service';

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

	localStorage.setItem('CONVOY_AUTH', JSON.stringify(res.data));
	// TODO set type for res.data
	// @ts-expect-error coming to this soonest TODO
	localStorage.setItem('CONVOY_AUTH_TOKENS', JSON.stringify(res.data.token));

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
	const res = await deps.httpReq<{ redirectUrl: string }>({
		url: '/auth/sso',
		method: 'get',
	});

	return res;
}
