import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { PrivateService } from 'src/app/private/private.service';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class EndpointDetailsService {
	constructor(private http: HttpService, private privateService: PrivateService) {}

	getEndpoint(endpointId: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${this.privateService.urlFactory('org_project')}/endpoints/${endpointId}`,
					method: 'get'
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
					url: `${this.privateService.urlFactory('org_project')}/endpoints/${endpointId}`,
					method: 'delete'
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
