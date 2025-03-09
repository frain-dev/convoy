import axios from 'axios';
import { router } from '../lib/router';
import { isProductionMode } from '@/lib/env';

import * as authService from '@/services/auth.service';
import * as projectsService from '@/services/projects.service';

import type { HttpResponse } from '@/models/global.model';
import { useOrganisationStore } from '@/store';

const APIURL = `${isProductionMode ? location.origin : 'http://localhost:5005'}/ui`;
const APP_PORTAL_APIURL = `${isProductionMode ? location.origin : 'http://localhost:5005'}/portal-api`;

function getToken() {
	const token = router.state.location.search?.token as string | undefined;
	return token ? token : '';
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

export function buildRequestPath(
	level?: 'org' | 'org_project',
	deps: {
		getCachedProject: typeof projectsService.getCachedProject;
		getCachedOrganisationId: () => string;
	} = {
		getCachedProject: projectsService.getCachedProject,
		getCachedOrganisationId: () => {
			const {org} = useOrganisationStore.getState()
			return org?.uid || ''
		},
	},
): string {
	if (!level) return '';
	const orgId = deps.getCachedOrganisationId();
	const projectId = deps.getCachedProject()?.uid;

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

export function setupAxios(
	requestDetails: { hideNotification?: boolean },
	deps: {
		logUserOut: typeof authService.logUserOut;
	} = { logUserOut: authService.logUserOut },
) {
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
					deps.logUserOut();
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

export async function request<TData>(
	requestDetails: {
		url: string;
		body?: any;
		method: 'get' | 'post' | 'delete' | 'put';
		hideNotification?: boolean;
		query?: Record<string, any>;
		level?: 'org' | 'org_project';
		isOut?: boolean;
	},
	deps: { getAuthDetails: typeof authService.getCachedAuthTokens } = {
		getAuthDetails: authService.getCachedAuthTokens,
	},
): Promise<HttpResponse<TData>> {
	const url = buildURL(requestDetails);
	if (url == 'error') throw new Error('Error constructing URL');

	const { hideNotification, body, method } = requestDetails;
	const http = setupAxios({ hideNotification: !!hideNotification });

	const requestHeader = {
		Authorization: `Bearer ${getToken() || deps.getAuthDetails().access_token || ''}`,
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

		console.error('unexpected error:', error);

		throw new Error('An unexpected error occured');
	}
}
