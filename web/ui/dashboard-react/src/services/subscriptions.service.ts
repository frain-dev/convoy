import { request } from '@/services/http.service';
import type { PaginatedResult } from '@/models/global.model';
import type { SUBSCRIPTION } from '@/models/subscription.model';

type GetSubscriptionsReqDetails = {
  name?: string;
  endpointId?: string;
  next_page_cursor?: string;
  direction?: 'next' | 'prev';
}

export async function getSubscriptions(reqDetails: GetSubscriptionsReqDetails,
	deps: { httpReq: typeof request } = { httpReq: request },
) {
  if(!reqDetails.direction) reqDetails.direction = 'next'
  if(!reqDetails.next_page_cursor) reqDetails.next_page_cursor = 'FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF'
  
  const res = await deps.httpReq<PaginatedResult<SUBSCRIPTION>>({
    url: `/subscriptions`,
    method: 'get',
    level: 'org_project',
    // @ts-expect-error types match in reality
    query: reqDetails
  })

  return res.data
}
