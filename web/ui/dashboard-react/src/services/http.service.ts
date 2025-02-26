import axios from 'axios';
import { router } from '../lib/router';
import { isProductionMode } from '@/lib/env';
import { CONVOY_LAST_AUTH_LOCATION_KEY, CONVOY_ORG_KEY } from '@/lib/constants';

import type { HttpResponse } from '@/models/global.model';

const APIURL = `${isProductionMode ? location.origin : 'http://localhost:5005'}/ui`;
const APP_PORTAL_APIURL = `${isProductionMode ? location.origin : 'http://localhost:5005'}/portal-api`;

function getToken() {
	// @ts-expect-error with the ?. operator, we're fine here
	const token = router.state.location.search?.token as string | undefined;
	return token ? token : '';
}

type AuthDetailsTokenJson = {
	access_token: string;
	refresh_token: string;
};

export function authDetails() {
	const authDetails = localStorage.getItem('CONVOY_AUTH_TOKENS');

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

export function buildRequestQuery(
	query?: Record<string, string | number | object | undefined | null>,
) {
	if (!query || Object.getOwnPropertyNames(query).length == 0) return '';

	// add portal link query if available
	if (getToken()) query.token = getToken();

	// TODO check if there's a library that does this 👇🏽
	// remove empty data and objects in object
	const cleanedQuery = Object.fromEntries(
		Object.entries(query).filter(
			([, q]) =>
				q !== '' && q !== undefined && q !== null && typeof q !== 'object',
		),
	) as Record<string, string | number | object>;

	// convert object to query param
	let cleanedQueryString: string = '';
	Object.keys(cleanedQuery).forEach((q, i) => {
		try {
			const queryItem = JSON.parse(`${query[q]}`);
			queryItem.forEach(
				(item: string) =>
					(cleanedQueryString += `${q}=${item}${Object.keys(cleanedQuery).length - 1 !== i ? '&' : ''}`),
			);
		} catch (error) {
			cleanedQueryString += `${q}=${query[q] as string}${Object.keys(cleanedQuery).length - 1 !== i ? '&' : ''}`;
		}
	});

	// for query items with arrays, process them into a string
	let queryString = '';
	Object.keys(query).forEach((key: string) => {
		if (Array.isArray(query[key])) {
			query[key].forEach((item: string) => (queryString += `&${key}=${item}`));
		}
	});

	return cleanedQueryString + queryString;
}

export function getOrganisation() {
	const org = localStorage.getItem(CONVOY_ORG_KEY);
	return org ? (JSON.parse(org) as { name: string; uid: string }) : null;
}

export function getProject() {
	const project = localStorage.getItem('CONVOY_PROJECT');
	return project ? (JSON.parse(project) as { uid: string }) : null;
}

export function buildRequestPath(level?: 'org' | 'org_project'): string {
	if (!level) return '';
	const orgId = getOrganisation()?.uid;
	const projectId = getProject()?.uid;

	if (level == 'org' && orgId) return `/organisations/${orgId}`;

	if (level == 'org_project' && orgId && projectId) {
		return `/organisations/${orgId}/projects/${projectId}`;
	}

	const isError =
		(level === 'org' && !orgId) ||
		(level === 'org_project' && (!orgId || !projectId));

	if (isError) return 'error';

	return '';
}

export function buildURL(requestDetails: any): string {
	if (requestDetails.isOut) return requestDetails.url;

	if (getToken())
		return `${getToken() ? APP_PORTAL_APIURL : APIURL}${requestDetails.url}${requestDetails.query ? '?' + buildRequestQuery(requestDetails.query) : ''}`;

	if (!requestDetails.level)
		return `${APIURL}${requestDetails.url}${requestDetails.query ? '?' + buildRequestQuery(requestDetails.query) : ''}`;

	const requestPath = buildRequestPath(requestDetails.level);
	if (requestPath === 'error') return 'error';

	return `${APIURL}${requestPath}${requestDetails.url}${requestDetails.query ? '?' + buildRequestQuery(requestDetails.query) : ''}`;
}

export function setupAxios(requestDetails: { hideNotification?: boolean }) {
	const http = axios.create();

	http.interceptors.response.use(
		request => request,
		error => {
			if (axios.isAxiosError(error)) {
				const errorResponse = error.response;
				let errorMessage = errorResponse?.data
					? errorResponse.data.message
					: error.message;

				if (
					error.response?.status == 401 &&
					!router.state.location.pathname.startsWith('/portal')
				) {
					logUserOut();
					return Promise.reject(error);
				}

				if (!requestDetails.hideNotification) {
					// TODO GeneralService.showNotification; for now
					console.error(errorMessage);
				}

				return Promise.reject(error);
			}

			if (!requestDetails.hideNotification) {
				let errorMessage: string;
				error.error?.message
					? (errorMessage = error.error?.message)
					: (errorMessage = 'An error occured, please try again');
				// TODO GeneralService.showNotification; for now
				console.error(errorMessage);
			}

			return Promise.reject(error);
		},
	);

	return http;
}

export async function request<TData>(requestDetails: {
	url: string;
	body?: any;
	method: 'get' | 'post' | 'delete' | 'put';
	hideNotification?: boolean;
	query?: Record<string, any>;
	level?: 'org' | 'org_project';
	isOut?: boolean;
}): Promise<HttpResponse<TData>> {
	const url = buildURL(requestDetails);
	if (url == 'error') throw new Error('Error constructing URL');

	const { hideNotification, body, method } = requestDetails;
	const http = setupAxios({ hideNotification: !!hideNotification });

	const requestHeader = {
		Authorization: `Bearer ${getToken() || authDetails().access_token || ''}`,
		...(isProductionMode && { 'X-Convoy-Version': '2024-04-01' }), // TODO confirm from @RT if this is permitted on the server
	};

	try {
		const { data } = await http.request({
			url,
			data: body,
			method: method,
			headers: requestHeader,
		});

		return data;
	} catch (error) {
		if (axios.isAxiosError(error)) {
			throw new Error(error.message);
		}

		console.log('unexpected error:', error);

		throw new Error('An unexpected error occured');
	}
}

export function logUserOut() {
	// save previous location before session timeout
	if (!router.state.location.pathname.startsWith('/login')) {
		localStorage.setItem(CONVOY_LAST_AUTH_LOCATION_KEY, location.href);
	}

	// then move user to login page
	router.navigate({
		// @ts-expect-error `pathname` is definitely a route
		from: router.state.location.pathname,
		to: '/',
	});
}
