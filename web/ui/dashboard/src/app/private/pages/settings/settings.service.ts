import {Injectable} from '@angular/core';
import {HTTP_RESPONSE} from 'src/app/models/global.model';
import {HttpService} from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class SettingsService {
	constructor(private http: HttpService) {}

	updateOrganisation(requestDetails: { org_id: string; body: { name: string } }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/organisations/${requestDetails.org_id}`,
					method: 'put',
					body: requestDetails.body
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	deleteOrganisation(requestDetails: { org_id: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/organisations/${requestDetails.org_id}`,
					method: 'delete',
					body: null
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	fetchConfigSettings(): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/configuration`,
					method: 'get'
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	updateConfigSettings(requestDetails?: {}): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/configuration`,
					method: 'put',
					body: requestDetails
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getEarlyAdopterFeatures(requestDetails: { org_id: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/organisations/${requestDetails.org_id}/early-adopter-features`,
					method: 'get'
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	updateOrganisationFeatureFlags(requestDetails: { org_id: string; body: { feature_flags: { [key: string]: boolean } } }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/organisations/${requestDetails.org_id}/feature-flags`,
					method: 'put',
					body: requestDetails.body
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	checkFeatureFlagEnabled(requestDetails: { org_id: string; feature_key: string }): Promise<boolean> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.getEarlyAdopterFeatures({ org_id: requestDetails.org_id });
				const features = response.data || [];
				const feature = features.find((f: any) => f.key === requestDetails.feature_key);
				resolve(feature?.enabled || false);
			} catch (error) {
				resolve(false);
			}
		});
	}

	getOrganisationFeatureFlags(requestDetails: { org_id: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/organisations/${requestDetails.org_id}/feature-flags`,
					method: 'get'
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getSSOAdminPortal(returnUrl: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/sso/admin-portal`,
					method: 'post',
					body: { return_url: returnUrl },
					hideNotification: true
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
