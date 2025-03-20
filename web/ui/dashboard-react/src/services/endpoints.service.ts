import { request } from './http.service';
import type { ENDPOINT } from '../models/endpoint.model';
import type { HttpResponse } from '@/models/global.model';
// TODO: type these data properly
type RequestBody = Record<
	string,
	string | number | object | Record<string, string | number | object | null | undefined> | null | undefined
>;

export async function addEndpoint(
	body: Record<string, unknown>,
): Promise<HttpResponse<{ data: ENDPOINT; message: string }>> {
	return request<{ data: ENDPOINT; message: string }>({
		url: '/endpoints',
		method: 'post',
		body: body as RequestBody,
		level: 'org_project',
	});
}

export async function updateEndpoint(
	endpointId: string,
	body: Record<string, unknown>,
): Promise<HttpResponse<{ data: ENDPOINT; message: string }>> {
	return request<{ data: ENDPOINT; message: string }>({
		url: `/endpoints/${endpointId}`,
		method: 'put',
		body: body as RequestBody,
		level: 'org_project',
	});
}

export async function getEndpoint(
	endpointId: string,
): Promise<HttpResponse<{ data: ENDPOINT; message: string }>> {
	return request<{ data: ENDPOINT; message: string }>({
		url: `/endpoints/${endpointId}`,
		method: 'get',
		level: 'org_project',
	});
}

export async function getEndpoints(
	params: Record<string, string> = {},
): Promise<HttpResponse<{ data: { content: ENDPOINT[] }; message: string }>> {
	return request<{ data: { content: ENDPOINT[] }; message: string }>({
		url: '/endpoints',
		method: 'get',
		query: params as unknown as Record<string, Record<string, string | number | object | undefined | null>>,
		level: 'org_project',
	});
}

export async function deleteEndpoint(
	endpointId: string,
): Promise<HttpResponse<{ message: string }>> {
	return request<{ message: string }>({
		url: `/endpoints/${endpointId}`,
		method: 'delete',
		level: 'org_project',
	});
}

// Export all functions as an object for compatibility
export const endpointsService = {
	addEndpoint,
	updateEndpoint,
	getEndpoint,
	getEndpoints,
	deleteEndpoint
};
