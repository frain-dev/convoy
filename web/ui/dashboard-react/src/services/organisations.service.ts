import { request } from './http.service';
import { CONVOY_ORG_KEY } from '@/lib/constants';

import type { Organisation } from '@/models/organisation.model';

type Pagination = {
	per_page: number;
	has_next_page: boolean;
	has_prev_page: boolean;
	prev_page_cursor: string;
	next_page_cursor: string;
};

let organisations: Array<Organisation> = [];

export function getCachedOrganisation(): Organisation | null {
	let org = localStorage.getItem(CONVOY_ORG_KEY);
	return org ? JSON.parse(org) : null;
}

function setDefaultCachedOrganisation(organisations: Organisation[]) {
	if (!organisations?.length) return;

	const existingOrg = organisations.find(
		org => org.uid == getCachedOrganisation()?.uid,
	);

	if (existingOrg)
		return localStorage.setItem(CONVOY_ORG_KEY, JSON.stringify(existingOrg));

	localStorage.setItem(CONVOY_ORG_KEY, JSON.stringify(organisations[0]));
}

export function getDefaultCachedOrganisation() {
	const cached = localStorage.getItem(CONVOY_ORG_KEY);
	if (!cached) return null;

	const cachedOrg = JSON.parse(cached);
	return cachedOrg as Organisation;
}

type PaginatedOrganisationResult = {
	content: Array<Organisation>;
	pagination: Pagination;
};

export function getOrganisations(reqDetails: {
	refresh: true;
}): Promise<PaginatedOrganisationResult>;

export function getOrganisations(reqDetails: {
	refresh: false | undefined;
}): Promise<Array<Organisation>>;

export async function getOrganisations(
	reqDetails: { refresh?: boolean },
	deps: { httpReq: typeof request } = { httpReq: request },
) {
	if (!reqDetails.refresh) return organisations;

	const res = await deps.httpReq({
		url: '/organisations',
		method: 'get',
	});

	setDefaultCachedOrganisation(
		(res.data as PaginatedOrganisationResult).content,
	);

	return res.data;
}
