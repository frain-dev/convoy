import { request } from '@/services/http.service';
import type { PaginatedResult } from '@/models/global.model';
import type { SUBSCRIPTION } from '@/models/subscription.model';

type GetSubscriptionsReqDetails = {
  name?: string;
  endpointId?: string;
  next_page_cursor?: string;
  direction?: 'next' | 'prev';
}

// Subscription creation type
type CreateSubscriptionData = {
  name: string;
  endpoint_id: string;
  source_id?: string;
  filter_config?: {
    event_types?: string[] | null;
    filter?: {
      headers?: Record<string, string> | null;
      body?: Record<string, string> | null;
    };
  };
  function?: string | null;
};

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

// Add new subscription
export async function createSubscription(data: CreateSubscriptionData, 
  deps: { httpReq: typeof request } = { httpReq: request },
) {
  const res = await deps.httpReq<{data: SUBSCRIPTION}>({
    url: '/subscriptions',
    method: 'post',
    level: 'org_project',
    body: data
  });

  return res.data;
}
