import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { PrivateService } from 'src/app/private/private.service';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class EndpointsService {
	projectId = this.privateService.activeProjectDetails?.uid;

	constructor(private http: HttpService, private privateService: PrivateService) {}

	generateKey(requestDetails: { appId: string; body: { key_type: string; name?: string; expiration?: string } }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${this.privateService.urlFactory('org_project')}/keys`,
					method: 'post',
					body: requestDetails.body
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getEndpoints(requestDetails?: { pageNo?: number; searchString?: string; token?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: requestDetails?.token ? '/endpoints' : this.privateService.urlFactory('org_project') + `/endpoints?sort=AESC&page=${requestDetails?.pageNo || 1}&perPage=20${requestDetails?.searchString ? `&q=${requestDetails?.searchString}` : ''}`,
					method: 'get',
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
					url: `${this.privateService.urlFactory('org_project')}/endpoints/${requestDetails.endpointId}`,
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
					url: `${this.privateService.urlFactory('org_project')}/endpoints/${requestDetails.endpointId}/expire_secret`,
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
