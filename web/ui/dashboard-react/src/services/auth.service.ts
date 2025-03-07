import { request } from './http.service';
import {
	CONVOY_AUTH_KEY,
	CONVOY_AUTH_TOKENS_KEY,
	CONVOY_LAST_AUTH_LOCATION_KEY,
} from '@/lib/constants';
import { router } from '@/lib/router';
import * as orgsService from '@/services/organisations.service';

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

/** As a side effect, it sets the cached auth profile and the cached auth tokens */
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

/** As a side effect, it clears all cached items */
export function logUserOut() {
	// save previous location before session timeout
	if (!router.state.location.pathname.startsWith('/login')) {
		localStorage.clear();
		localStorage.setItem(CONVOY_LAST_AUTH_LOCATION_KEY, location.href);
	}

	// then move user to login page
	router.navigate({ to: '/' });
}

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

	localStorage.setItem(CONVOY_AUTH_KEY, JSON.stringify(res.data));
	// TODO set type for res.data
	// @ts-expect-error coming to this soonest TODO
	localStorage.setItem(CONVOY_AUTH_TOKENS_KEY, JSON.stringify(res.data.token));

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

const permissions = {
	MEMBER: [
		'Event Deliveries|VIEW',
		'Event Deliveries|MANAGE',
		'Sources|VIEW',
		'Subscriptions|VIEW',
		'Endpoints|VIEW',
		'Portal Links|VIEW',
		'Events|VIEW',
		'Events|MANAGE',
		'Meta Events|VIEW',
		'Project Settings|VIEW',
		'Projects|VIEW',
		'Team|VIEW',
		'Organisations|VIEW',
	],
	SUPER_ADMIN: ['Team|MANAGE', 'Organisations|MANAGE'],
	ADMIN: [
		'Sources|MANAGE',
		'Subscriptions|MANAGE',
		'Endpoints|MANAGE',
		'Portal Links|MANAGE',
		'Meta Events|MANAGE',
		'Project Settings|MANAGE',
		'Projects|MANAGE',
	],
};

type Permission =
	| 'Event Deliveries|VIEW'
	| 'Event Deliveries|MANAGE'
	| 'Sources|VIEW'
	| 'Sources|MANAGE'
	| 'Subscriptions|VIEW'
	| 'Subscriptions|MANAGE'
	| 'Endpoints|VIEW'
	| 'Endpoints|MANAGE'
	| 'Portal Links|VIEW'
	| 'Portal Links|MANAGE'
	| 'Events|VIEW'
	| 'Events|MANAGE'
	| 'Meta Events|VIEW'
	| 'Meta Events|MANAGE'
	| 'Project Settings|VIEW'
	| 'Project Settings|MANAGE'
	| 'Projects|VIEW'
	| 'Projects|MANAGE'
	| 'Team|VIEW'
	| 'Team|MANAGE'
	| 'Organisations|VIEW'
	| 'Organisations|MANAGE';

export async function getUserRole(
	reqDetails: Parameters<typeof orgsService.getTeamMembers>[0],
	deps: {
		httpReq: typeof request;
		getTeamMembers: typeof orgsService.getTeamMembers;
	} = { httpReq: request, getTeamMembers: orgsService.getTeamMembers },
) {
	try {
		const { content } = await deps.getTeamMembers(reqDetails, deps);
		const role = content[0].role.type;

		switch (role) {
			case 'super_user':
				return 'SUPER_ADMIN';
			case 'admin':
				return 'ADMIN';
			default:
				return 'MEMBER';
		}
	} catch (error) {
		return 'MEMBER';
	}
}

async function getUserPermissions() {
	const role = await getUserRole({ userID: getCachedAuthProfile()?.uid });

	switch (role) {
		case 'SUPER_ADMIN':
			return permissions[role].concat(permissions.ADMIN, permissions.MEMBER);
		case 'ADMIN':
			return permissions[role].concat(permissions.MEMBER);
		default:
			return permissions.MEMBER;
	}
}

export async function ensureUserCanAccess(
	reqPermission: Permission,
	token?: string,
) {
	const permissions = await getUserPermissions();
	const portalPermissions = ['Subscriptions|MANAGE', 'Endpoints|MANAGE'];

	if (token) return portalPermissions.includes(reqPermission);
	return permissions.includes(reqPermission);
}
