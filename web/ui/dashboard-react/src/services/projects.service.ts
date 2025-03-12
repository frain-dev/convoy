import { request } from '@/services/http.service';

import type { Project, CreateProjectResponse } from '@/models/project.model';

type CreateProjectParams = {
	name: string;
	type: 'incoming' | 'outgoing';
	config: {
		strategy?: {
			duration: number;
			retry_count: number;
			type: 'linear' | 'exponential';
		};
		signature?: {
			header: string;
			versions: Array<{
				hash: 'SHA256' | 'SHA512';
				encoding: 'base64' | 'hex';
			}>;
		};
		ratelimit?: {
			count: number;
			duration: number;
		};
		search_policy?: `${string}h`;
	};
};

export async function createProject(
	reqDetails: CreateProjectParams,
	deps: { httpReq: typeof request } = { httpReq: request },
) {
	const response = await deps.httpReq<CreateProjectResponse>({
		url: `/projects`,
		body: reqDetails,
		method: 'post',
		level: 'org',
	});

	return response.data;
}

export async function getProjects(
	deps: { httpReq: typeof request } = { httpReq: request },
) {
	const res = await deps.httpReq<Array<Project>>({
		url: '/projects',
		method: 'get',
		level: 'org',
	});

	return res.data;
}

export async function getProject(
	projectId: string,
	deps: { httpReq: typeof request } = { httpReq: request },
) {
	const res = await deps.httpReq<Project>({
		url: `/projects/${projectId}`,
		method: 'get',
		level: 'org',
	});

	return res.data;
}
