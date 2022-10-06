import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { HttpService } from 'src/app/services/http/http.service';

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

	updateConfigSettings(requestDetails: {}): Promise<HTTP_RESPONSE> {
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

	fetchPersonalKeys(requestDetails: { userId: string; pageNo: number }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/users/${requestDetails.userId}/security/personal_api_keys?sort=AESC&page=${requestDetails?.pageNo || 1}&perPage=10`,
					method: 'get'
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	generatePersonalKey(userId: string, requestDetails: { name: string; expiration: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/users/${userId}/security/personal_api_keys`,
					method: 'post',
					body: requestDetails
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	revokeKey(requestDetails: { userId: string; keyId: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/users/${requestDetails.userId}/security/personal_api_keys/${requestDetails.keyId}/revoke`,
					method: 'put',
                    body: null
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
