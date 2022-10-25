import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { PrivateService } from 'src/app/private/private.service';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class AppDetailsService {
	projectId = this.privateService.activeProjectDetails?.uid;

	constructor(private http: HttpService, private privateService: PrivateService) {}

	getApps(): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${this.privateService.urlFactory('org_project')}/apps?&sort=AESC&page=1&perPage=50`,
					method: 'get'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	generateKey(requestDetails: { appId: string; body: { key_type: string; name?: string; expiration?: string } }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${this.privateService.urlFactory('org_project')}/apps/${requestDetails.appId}/keys`,
					method: 'post',
					body: requestDetails.body
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getApp(appId: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${this.privateService.urlFactory('org_project')}/apps/${appId}`,
					method: 'get'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	addNewEndpoint(requestDetails: { appId: string; body: any; token?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: requestDetails.token ? '/apps/endpoints' : this.privateService.urlFactory('org_project') + `/apps/${requestDetails.appId}/endpoints`,
					body: requestDetails.body,
					method: 'post',
					token: requestDetails?.token
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	editEndpoint(requestDetails: { appId: string; endpointId: string; body: any; token?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${requestDetails.token ? '' : this.privateService.urlFactory('org_project')}/apps/${requestDetails.appId}/endpoints/${requestDetails.endpointId}`,
					body: requestDetails.body,
					method: 'put',
					token: requestDetails?.token
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	deleteEndpoint(requestDetails: { appId: string; endpointId: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${this.privateService.urlFactory('org_project')}/apps/${requestDetails.appId}/endpoints/${requestDetails.endpointId}`,
					method: 'delete'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	expireSecret(requestDetails: { appId: string; endpointId: string; body: { expiration: number } }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${this.privateService.urlFactory('org_project')}/apps/${requestDetails.appId}/endpoints/${requestDetails.endpointId}/expire_secret`,
					method: 'put',
                    body: requestDetails.body
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	sendEvent(requestDetails: { body: any }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${this.privateService.urlFactory('org_project')}/events`,
					body: requestDetails.body,
					method: 'post'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
