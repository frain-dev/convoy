import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class CliKeysService {
	constructor(private http: HttpService) {}

	generateKey(requestDetails: { endpointId: string; body: { key_type: string; name?: string; expiration?: string } }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/endpoints/${requestDetails.endpointId}/keys`,
					method: 'post',
					body: requestDetails.body,
					level: 'org_project'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getApiKeys(requestDetails: { endpointId: string; token?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: requestDetails.token ? `/keys` : `/endpoints/${requestDetails.endpointId}/keys`,
					method: 'get',
					level: 'org_project'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	revokeApiKey(requestDetails: { endpointId: string; keyId: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/endpoints/${requestDetails.endpointId}/keys/${requestDetails.keyId}/revoke`,
					method: 'put',
					body: null,
					level: 'org_project'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
