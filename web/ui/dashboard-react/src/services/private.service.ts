import { request } from './http.service';

import type { ORGANIZATION_DATA } from '@/models/organisation.model';

let organisations: Array<ORGANIZATION_DATA> = [];
let organisationDetails: ORGANIZATION_DATA | undefined = undefined;

function getCachedOrganisation(): ORGANIZATION_DATA | null {
	let org = localStorage.getItem('CONVOY_ORG');
	return org ? JSON.parse(org) : null;
}

function setOrganisationConfig(organisations: ORGANIZATION_DATA[]) {
	if (!organisations?.length) return;

	const existingOrg = organisations.find(
		org => org.uid == getCachedOrganisation()?.uid,
	);

	if (existingOrg)
		return localStorage.setItem('CONVOY_ORG', JSON.stringify(existingOrg));

	organisationDetails = organisations[0];
	console.log(organisationDetails) // TODO remove this line when you read this variable
	localStorage.setItem('CONVOY_ORG', JSON.stringify(organisations[0]));
}

export async function getOrganisations(
	{ refresh }: Partial<{ refresh: boolean }>,
	deps: { httpReq: typeof request } = { httpReq: request },
) {
	if (organisations?.length && !refresh) return organisations;

	const res = await deps.httpReq<{ content: Array<ORGANIZATION_DATA> }>({
		url: '/organisations',
		method: 'get',
	});

	setOrganisationConfig(res.data.content);
	// @ts-expect-error TODO check what the response is here to be sure as it may be different from res.data.content
	organisations = res;
	return res;
}
