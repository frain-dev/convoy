import { request } from '@/services/http.service';

import type { SOURCE, CreateSourceResponseData } from '@/models/source';
import type { PaginatedResult, PaginationCursor } from '@/models/global.model';

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

type GetSourcesReqDetails = PaginationCursor & { q?: string };

export async function getSources(
	reqDetails?: GetSourcesReqDetails,
	deps: { httpReq: typeof request } = { httpReq: request },
) {
	if (!reqDetails)
		reqDetails = {
			next_page_cursor: 'FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF',
			direction: 'next',
		};

	const res = await deps.httpReq<PaginatedResult<SOURCE>>({
		url: `/sources`,
		method: 'get',
		level: 'org_project',
		// @ts-expect-error types match in reality
		query: reqDetails,
	});

	return res.data;
}

// Get source details
export async function getSourceDetails(
	sourceId: string,
	deps: { httpReq: typeof request } = { httpReq: request },
) {
	const res = await deps.httpReq<SOURCE>({
		url: `/sources/${sourceId}`,
		method: 'get',
		level: 'org_project',
	});

	return res.data;
}

export async function createSource(
	reqDetails: any, // TODO update this type
	deps: { httpReq: typeof request } = { httpReq: request },
) {
	const res = await deps.httpReq<CreateSourceResponseData>({
		url: `/sources`,
		method: 'post',
		body: reqDetails,
		level: 'org_project',
	});

	return res.data;
}

type TestTransformFunctionResponse = {
	payload: {
		custom_headers: Record<string, string>;
		data: any;
	};
	log: string[];
};

/**
 * Test a transform function against a payload
 * @param data Object containing the function code and test payload
 */
export async function testTransformFunction(
	reqDetails: {
		payload: Record<string, unknown>;
		function: string;
		type?: 'body' | 'header';
	},
	deps: { httpReq: typeof request } = { httpReq: request },
) {
	const res = await deps.httpReq<TestTransformFunctionResponse>({
		url: '/sources/test_function',
		method: 'post',
		body: reqDetails,
		level: 'org_project',
	});

	return res.data;
}

export async function deleteSource(
	sourceId: string,
	deps: { httpReq: typeof request } = { httpReq: request },
) {
	const res = await deps.httpReq<null>({
		url: `/sources/${sourceId}`,
		method: 'delete',
		level: 'org_project',
	});

	return res.data;
}
