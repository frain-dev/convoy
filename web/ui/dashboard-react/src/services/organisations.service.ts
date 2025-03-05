import { request } from './http.service';

import type { Organisation } from '@/models/organisation.model';

// TODO move some of htese to a use-organisation hook

type Pagination = {
	per_page: number;
	has_next_page: boolean;
	has_prev_page: boolean;
	prev_page_cursor: string;
	next_page_cursor: string;
};

let organisations: Array<Organisation> = [];

// export function getCachedOrganisation(): Organisation | null {
// 	let org = localStorage.getItem(CONVOY_ORG_KEY);
// 	return org ? JSON.parse(org) : null;
// }

// export function setDefaultCachedOrganisation(organisations: Organisation[]) {
// 	if (!organisations.length) return localStorage.removeItem(CONVOY_ORG_KEY);

// 	return localStorage.setItem(CONVOY_ORG_KEY, JSON.stringify(organisations[0]));
// }

// export function getDefaultCachedOrganisation() {
// 	const cached = localStorage.getItem(CONVOY_ORG_KEY);
// 	if (!cached) return null;

// 	const cachedOrg = JSON.parse(cached);
// 	return cachedOrg as Organisation;
// }

type PaginatedOrganisationResult = {
	content: Array<Organisation>;
	pagination: Pagination;
};

/**
 * As a side effect, it sets the default cached organisation
 */
export function getOrganisations(reqDetails: {
	refresh: true;
}): Promise<PaginatedOrganisationResult>;

/**
 * As a side effect, it sets the default cached organisation
 */
export function getOrganisations(reqDetails: {
	refresh: false | undefined;
}): Promise<Array<Organisation>>;

export async function getOrganisations(
	reqDetails: { refresh?: boolean },
	deps: { httpReq: typeof request } = { httpReq: request },
) {
	if (!reqDetails.refresh) return organisations;

	const res = await deps.httpReq<PaginatedOrganisationResult>({
		url: '/organisations',
		method: 'get',
	});

	organisations = res.data.content

	return res.data;
}

type AddOrganisationParams = {
	name: string;
};
export async function addOrganisation(
	reqDetails: AddOrganisationParams,
	deps: { httpReq: typeof request } = { httpReq: request },
) {
	const res = await deps.httpReq({
		method: 'post',
		url: '/organisations',
		body: reqDetails,
	});

	return res.data;
}
