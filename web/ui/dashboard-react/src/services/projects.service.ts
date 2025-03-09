import { request } from '@/services/http.service';
import { CONVOY_CURRENT_PROJECT } from '@/lib/constants';

import type { Project } from '@/models/project.model';

// TODO use state management
let projects: Array<Project> = [];
let projectDetails: Project | null = null;

export async function getProjects(
	reqDetails: { refresh?: boolean },
	deps: { httpReq: typeof request } = { httpReq: request },
) {
	if (projects.length && !reqDetails.refresh) return projects;

	const res = await deps.httpReq<Array<Project>>({
		url: '/projects',
		method: 'get',
		level: 'org',
	});

	return res.data;
}

export async function getProject(
	reqDetails: { refresh?: boolean; projectId: string },
	deps: { httpReq: typeof request } = { httpReq: request },
) {
	if (projectDetails && !reqDetails.refresh) return projectDetails;

	const res = await deps.httpReq<Project>({
		url: `/projects/${reqDetails.projectId}`,
		method: 'get',
		level: 'org',
	});

	projectDetails = res.data;

	return res;
}

export function getCachedProject(): Project | null {
	const cachedProject = localStorage.getItem(CONVOY_CURRENT_PROJECT);
	return cachedProject && cachedProject != 'undefined'
		? JSON.parse(cachedProject)
		: null;
}

export function setCachedProject(project: Project | null) {
	localStorage.setItem(CONVOY_CURRENT_PROJECT, JSON.stringify(project));
}
