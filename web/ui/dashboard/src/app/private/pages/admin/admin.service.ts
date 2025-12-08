import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/global.model';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class AdminService {
	constructor(private http: HttpService) {}

	getAllFeatureFlags(): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/admin/feature-flags`,
					method: 'get'
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getAllOrganisations(requestDetails?: { page?: number; perPage?: number; search?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const params = new URLSearchParams();
				if (requestDetails?.page) params.append('page', requestDetails.page.toString());
				if (requestDetails?.perPage) params.append('perPage', requestDetails.perPage.toString());
				if (requestDetails?.search) params.append('search', requestDetails.search);
				
				const queryString = params.toString();
				const url = `/admin/organisations${queryString ? '?' + queryString : ''}`;
				
				const response = await this.http.request({
					url: url,
					method: 'get'
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getOrganisationOverrides(orgID: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/admin/organisations/${orgID}/overrides`,
					method: 'get'
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	updateOrganisationOverride(orgID: string, featureKey: string, enabled: boolean): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/admin/organisations/${orgID}/overrides`,
					method: 'put',
					body: {
						feature_key: featureKey,
						enabled: enabled
					}
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	deleteOrganisationOverride(orgID: string, featureKey: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/admin/organisations/${orgID}/overrides/${featureKey}`,
					method: 'delete'
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getOrganisationCircuitBreakerConfig(orgID: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/admin/organisations/${orgID}/circuit-breaker-config`,
					method: 'get'
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getOrganisationProjects(orgID: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/admin/organisations/${orgID}/projects`,
					method: 'get'
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getProjectCircuitBreakerConfig(projectID: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/admin/projects/${projectID}/circuit-breaker-config`,
					method: 'get'
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	updateProjectCircuitBreakerConfig(projectID: string, config: {
		sample_rate: number;
		error_timeout: number;
		failure_threshold: number;
		success_threshold: number;
		observability_window: number;
		minimum_request_count: number;
		consecutive_failure_threshold: number;
	}): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/admin/projects/${projectID}/circuit-breaker-config`,
					method: 'put',
					body: config
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	updateOrganisationCircuitBreakerConfig(orgID: string, config: {
		failure_threshold: number;
		success_threshold: number;
		observability_window: number;
		minimum_request_count: number;
		consecutive_failure_threshold: number;
	}): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/admin/organisations/${orgID}/circuit-breaker-config`,
					method: 'put',
					body: config
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	updateFeatureFlag(featureKey: string, enabled?: boolean): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const body: any = { feature_key: featureKey };
				if (enabled !== undefined) body.enabled = enabled;

				const response = await this.http.request({
					url: `/admin/feature-flags/${featureKey}`,
					method: 'put',
					body: body
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	retryEventDeliveries(request: {
		project_id: string;
		status: string;
		time: string;
		event_id?: string;
	}): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const body: any = {
					project_id: request.project_id,
					status: request.status,
					time: request.time
				};
				if (request.event_id) {
					body.event_id = request.event_id;
				}

				const response = await this.http.request({
					url: `/admin/retry-event-deliveries`,
					method: 'post',
					body: body
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
