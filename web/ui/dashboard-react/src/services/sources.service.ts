import {request} from '@/services/http.service'

import type { SOURCE } from "@/models/source"
import type { PaginatedResult, PaginationCursor } from '@/models/global.model'

type GetSourcesReqDetails = PaginationCursor & { q?: string }

// Type for creating a source
export type CreateSourceData = {
  name: string;
  type?: string;
  is_disabled?: boolean;
  verifier?: {
    type: string;
    hmac?: {
      encoding?: string;
      hash?: string;
      header?: string;
      secret?: string;
    };
    basic_auth?: {
      username?: string;
      password?: string;
    };
    api_key?: {
      header_name?: string;
      header_value?: string;
    };
  };
  provider?: string;
  custom_response?: {
    body?: string;
    content_type?: string;
  };
  idempotency_keys?: string[];
  pub_sub?: {
    type?: string;
    workers?: number;
    google?: {
      service_account?: string;
      subscription_id?: string;
      project_id?: string;
    };
    sqs?: {
      queue_name?: string;
      access_key_id?: string;
      secret_key?: string;
      default_region?: string;
    };
    amqp?: {
      schema?: string;
      host?: string;
      port?: string;
      queue?: string;
      deadLetterExchange?: string | null;
      vhost?: string;
      auth?: {
        user?: string | null;
        password?: string | null;
      };
      bindExchange?: {
        exchange?: string | null;
        routingKey?: string;
      };
    };
    kafka?: {
      brokers?: string[];
      consumer_group_id?: string;
      topic_name?: string;
      auth?: {
        type?: string;
        tls?: boolean;
        username?: string;
        password?: string;
        hash?: string;
      };
    };
  };
};

export async function getSources(reqDetails: GetSourcesReqDetails,
	deps: { httpReq: typeof request } = { httpReq: request },
) {
  const res = await deps.httpReq<PaginatedResult<SOURCE>>({
    url: `/sources`,
    method: 'get',
    level: 'org_project',
    // @ts-expect-error types match in reality
    query: reqDetails
  })

  return res.data
}

// Create a new source
export async function createSource(data: CreateSourceData,
  deps: { httpReq: typeof request } = { httpReq: request },
) {
  const res = await deps.httpReq<{ data: SOURCE }>({
    url: '/sources',
    method: 'post',
    level: 'org_project',
    body: data
  });

  return res.data;
}

// Get source details
export async function getSourceDetails(sourceId: string,
  deps: { httpReq: typeof request } = { httpReq: request },
) {
  const res = await deps.httpReq<{ data: SOURCE }>({
    url: `/sources/${sourceId}`,
    method: 'get',
    level: 'org_project'
  });

  return res.data;
}
