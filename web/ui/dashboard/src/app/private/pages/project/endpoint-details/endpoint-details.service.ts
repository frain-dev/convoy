import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class EndpointDetailsService {
	constructor(private http: HttpService) {}

	getEndpoint(endpointId: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/endpoints/${endpointId}`,
					method: 'get',
					level: 'org_project'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	deleteEndpoint(endpointId: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/endpoints/${endpointId}`,
					method: 'delete',
					level: 'org_project'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	expireSecret(requestDetails: { endpointId: string; body: { expiration: number } }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/endpoints/${requestDetails.endpointId}/expire_secret`,
					method: 'put',
					body: requestDetails.body,
					level: 'org_project'
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
					url: `/events`,
					body: requestDetails.body,
					method: 'post',
					level: 'org_project'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	toggleEndpoint(endpointId: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/endpoints/${endpointId}/toggle_status`,
					method: 'put',
					level: 'org_project'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
