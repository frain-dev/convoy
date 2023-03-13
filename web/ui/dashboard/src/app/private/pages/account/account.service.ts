import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class AccountService {
	constructor(private http: HttpService) {}

	async logout(): Promise<HTTP_RESPONSE> {
		try {
			const response = await this.http.request({
				url: '/auth/logout',
				method: 'post',
				body: null
			});
			return response;
		} catch (error: any) {
			return error;
		}
	}

	editBasicInfo(requestDetails: { userId: string; body: { first_name: string; last_name: string; email: string } }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/users/${requestDetails.userId}/profile`,
					body: requestDetails.body,
					method: 'put'
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	changePassword(requestDetails: { userId: string; body: { current_password: string; password: string; password_confirmation: string } }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/users/${requestDetails.userId}/password`,
					body: requestDetails.body,
					method: 'put'
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	fetchPersonalKeys(requestDetails: { userId: string; page: number }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/users/${requestDetails.userId}/security/personal_api_keys`,
					method: 'get',
					query: { keyType: 'personal_key', page: requestDetails.page }
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
