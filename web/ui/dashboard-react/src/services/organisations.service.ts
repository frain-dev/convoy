import { request } from './http.service';

import type { PaginatedResult } from '@/models/global.model';
import type { Member, Organisation } from '@/models/organisation.model';

export async function getOrganisations(
	deps: { httpReq: typeof request } = {
		httpReq: request,
	},
) {
	const res = await deps.httpReq<PaginatedResult<Organisation>>({
		url: '/organisations',
		method: 'get',
	});

	return res.data;
}

type AddOrganisationParams = {
	name: string;
};

export async function addOrganisation(
	reqDetails: AddOrganisationParams,
	deps: { httpReq: typeof request } = {
		httpReq: request,
	},
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
	deps: { httpReq: typeof request } = {
		httpReq: request,
	},
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
	deps: { httpReq: typeof request } = {
		httpReq: request,
	},
) {
	const res = await deps.httpReq<null>({
		url: `/organisations/${orgId}`,
		method: 'delete',
		body: null,
	});

	return res.data;
}

export async function getTeamMembers(
	reqDetails: { q?: string; page?: number; userID?: string },
	deps: { httpReq: typeof request } = {
		httpReq: request,
	},
) {
	const res = await deps.httpReq<PaginatedResult<Member>>({
		url: `/members`,
		method: 'get',
		level: 'org',
		query: reqDetails,
	});

	return res.data;
}
