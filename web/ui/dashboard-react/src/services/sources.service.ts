import { request } from '@/services/http.service';

import type { CreateSourceResponseData } from '@/models/source';

export async function createSource(
	reqDetails: any, // TODO update this type
	deps: { httpReq: typeof request } = { httpReq: request },
) {
	const res = await  deps.httpReq<CreateSourceResponseData>({
		url: `/sources`,
		method: 'post',
		body: reqDetails,
		level: 'org_project',
	});

  return res.data
}
