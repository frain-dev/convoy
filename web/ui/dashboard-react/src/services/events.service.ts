import { request } from './http.service';

import type { Event } from '@/models/event';
import type { PaginatedResult, PaginationCursor } from '@/models/global.model';

type GetEventsParams = {
	page?: number;
	idempotencyKey?: string;
	startDate?: string;
	endDate?: string;
	query?: string;
	sourceId?: string;
	endpointId?: string;
} & PaginationCursor;

export async function getEvents(
	reqDetails?: GetEventsParams,
	deps: { httpReq: typeof request } = {
		httpReq: request,
	},
) {
	const res = await deps.httpReq<PaginatedResult<Event>>({
		url: `/events`,
		method: 'get',
		// @ts-expect-error it checks out
		query: reqDetails,
		level: 'org_project',
	});

	return res.data;
}
