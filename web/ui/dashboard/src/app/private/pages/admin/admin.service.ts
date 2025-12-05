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

	updateFeatureFlag(featureKey: string, enabled?: boolean, allowOverride?: boolean): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const body: any = { feature_key: featureKey };
				if (enabled !== undefined) body.enabled = enabled;
				if (allowOverride !== undefined) body.allow_override = allowOverride;

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
}
