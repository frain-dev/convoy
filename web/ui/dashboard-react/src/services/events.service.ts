import { request } from './http.service';

import type { Event, EventDelivery } from '@/models/event';
import type { PaginatedResult, PaginationCursor } from '@/models/global.model';

type GetEventsParams = {
	page?: number;
	idempotencyKey?: string;
	startDate?: string;
	endDate?: string;
	query?: string;
	sourceId?: string;
	endpointId?: string;
	showLoader?: boolean;
} & PaginationCursor;

type GetEventDeliveriesParams = {
	page?: number;
	startDate?: string;
	endDate?: string;
	endpointId?: string;
	eventId?: string;
	sourceId?: string;
	status?: string;
	next_page_cursor?: string;
	sort?: string;
};

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

export async function getEventDeliveries(
	reqDetails?: GetEventDeliveriesParams,
	deps: { httpReq: typeof request } = {
		httpReq: request,
	},
) {
	const res = await deps.httpReq<PaginatedResult<EventDelivery>>({
		url: `/eventdeliveries`,
		method: 'get',
		query: reqDetails,
		level: 'org_project',
	});

	return res.data;
}
