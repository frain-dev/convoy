import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { PrivateService } from 'src/app/private/private.service';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class CliKeysService {
	constructor(private http: HttpService, private privateService: PrivateService) {}

	generateKey(requestDetails: { appId: string; body: { key_type: string; name?: string; expiration?: string }; token?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: requestDetails.token ? '/apps/keys' : `${this.privateService.urlFactory('org_project')}/apps/${requestDetails.appId}/keys`,
					method: 'post',
					body: requestDetails.body,
					token: requestDetails.token
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getApiKeys(requestDetails: { appId: string; token?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: requestDetails.token ? '/apps/keys' : `${this.privateService.urlFactory('org_project')}/apps/${requestDetails.appId}/keys`,
					method: 'get',
					token: requestDetails.token
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	revokeApiKey(requestDetails: { appId: string; keyId: string; token?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: requestDetails.token ? `/apps/keys/${requestDetails.keyId}/revoke` : `${this.privateService.urlFactory('org_project')}/apps/${requestDetails.appId}/keys/${requestDetails.keyId}/revoke`,
					method: 'put',
					body: null,
					token: requestDetails.token
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	async getAppPortalApp(token: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/apps`,
					method: 'get',
					token
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
