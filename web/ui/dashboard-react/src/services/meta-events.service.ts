import { request } from './http.service';

import type { PaginatedResult, PaginationCursor } from '@/models/global.model';

interface MetaEvent {
	uid: string;
	status: string;
	event_type: string;
	metadata: {
		num_trials: number;
		data: string;
	};
	created_at: string;
	attempt: {
		request_http_header: string;
		response_http_header: string;
	};
}

export async function getMetaEvents(
	reqDetails?: PaginationCursor,
	deps: { httpReq: typeof request } = {
		httpReq: request,
	},
) {
	if (!reqDetails)
		reqDetails = {
			next_page_cursor: 'FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF',
			direction: 'next',
		};

	const res = await deps.httpReq<PaginatedResult<MetaEvent>>({
		url: `/meta-events`,
		method: 'get',
		// @ts-expect-error it works fine
		query: reqDetails,
		level: 'org_project',
	});

	return res.data;
}

export async function retryEvent(
	eventId: string,
	deps: { httpReq: typeof request } = {
		httpReq: request,
	},
) {
	const res = await deps.httpReq<MetaEvent>({
		url: `/meta-events/${eventId}/resend`,
		method: 'put',
		body: null,
		level: 'org_project',
	});

	return res.data;
}
