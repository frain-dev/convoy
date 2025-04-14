import { request } from '@/services/http.service';

import type {
	Project,
	CreateProjectResponse,
	EventType,
} from '@/models/project.model';

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

type UpdateProjectParams = {
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
		disable_endpoint: boolean;
		multiple_endpoint_subscriptions: boolean;
		ssl?: {
			enforce_secure_endpoints: boolean;
		};
		meta_event?: {
			event_type: string[] | null;
			is_enabled: boolean;
			secret: string;
			type: string;
			url: string;
		};
	};
};

export async function updateProject(
	update: UpdateProjectParams,
	deps: { httpReq: typeof request } = { httpReq: request },
) {
	const res = await deps.httpReq<Project>({
		method: 'put',
		url: '',
		body: update,
		level: 'org_project',
	});

	return res.data;
}

export async function deleteProject(
	_uid: string,
	deps: { httpReq: typeof request } = { httpReq: request },
) {
	const res = await deps.httpReq<null>({
		url: '',
		method: 'delete',
		level: 'org_project',
	});

	return res.data;
}

export async function getEventTypes(
	_projectId: string,
	deps: { httpReq: typeof request } = { httpReq: request },
) {
	const res = await deps.httpReq<{ event_types: Array<EventType> }>({
		method: 'get',
		url: '/event-types',
		level: 'org_project',
	});

	return res.data;
}

type CreateEventTypeParams = {
	name: string;
	category?: string;
	description?: string;
};

export async function createEventType(
	_projectId: string,
	eventType: CreateEventTypeParams,
	deps: { httpReq: typeof request } = { httpReq: request },
) {
	const res = await deps.httpReq<{ event_type: EventType }>({
		url: `/event-types`,
		method: 'post',
		body: eventType,
		level: 'org_project',
	});

	return res.data.event_type;
}

export async function deprecateEventType(
	eventUid: string,
	deps: { httpReq: typeof request } = { httpReq: request },
) {
	const res = await deps.httpReq<null>({
		url: `/event-types/${eventUid}/deprecate`,
		method: 'post',
		body: {},
		level: 'org_project',
	});

	return res;
}

export async function regenerateAPIKey(
	_projectId: string,
	deps: { httpReq: typeof request } = { httpReq: request },
) {
	const res = await deps.httpReq<{ key: string; uid: string }>({
		url: `/security/keys/regenerate`,
		method: 'put',
		body: null,
		level: 'org_project',
	});

	return res.data;
}

export type ProjectStats = {
	endpoints_exist: boolean;
	events_exist: boolean;
	sources_exist: boolean;
	subscriptions_exist: boolean;
};

export async function getStats(
	deps: { httpReq: typeof request } = { httpReq: request },
) {
	const res = await deps.httpReq<ProjectStats>({
		url: `/stats`,
		method: 'get',
		level: 'org_project',
	});

	return res.data;
}
