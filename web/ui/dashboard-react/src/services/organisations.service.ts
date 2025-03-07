import { request } from './http.service';

import type { Member, Organisation } from '@/models/organisation.model';

// TODO move some of htese to a use-organisation hook

type Pagination = {
	per_page: number;
	has_next_page: boolean;
	has_prev_page: boolean;
	prev_page_cursor: string;
	next_page_cursor: string;
};

let organisations: Array<Organisation> = [];

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

	organisations = res.data.content;

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

type UpdateOrganisationParams = {
	name: string;
	orgId: string;
};

export async function updateOrganisation(
	reqDetails: UpdateOrganisationParams,
	deps: { httpReq: typeof request } = { httpReq: request },
) {
	const res = await deps.httpReq<Organisation>({
		url: `/organisations/${reqDetails.orgId}`,
		method: 'put',
		body: { name: reqDetails.name },
	});

	return res.data;
}

export async function deleteOrganisation(
	orgId: string,
	deps: { httpReq: typeof request } = { httpReq: request },
) {
	const res = await deps.httpReq<null>({
		url: `/organisations/${orgId}`,
		method: 'delete',
		body: null,
	});

	return res.data;
}

type PaginatedMembersResult = {
	content: Array<Member>;
	pagination: Pagination;
};

export async function getTeamMembers(
	reqDetails: { q?: string; page?: number; userID?: string },
	deps: { httpReq: typeof request } = { httpReq: request },
) {
	const res = await deps.httpReq<PaginatedMembersResult>({
		url: `/members`,
		method: 'get',
		level: 'org',
		query: reqDetails,
	});

	return res.data;
}
