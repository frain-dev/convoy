import { request } from '@/services/http.service'

import type { CreateSourceResponseData } from '@/models/source';

export async function getSources(query: string) {
	const res = await request(
		{
			url: '/sources',
			params: {
				q: query,
			},
		},
		{
			keepPreviousData: true,
		}
	)
	return res.data
}

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

type TestTransformFunctionResponse = {
	payload: {
		custom_headers: Record<string, string>
		data: any
	};
  log: string[]
}

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
