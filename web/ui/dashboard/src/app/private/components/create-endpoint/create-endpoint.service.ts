import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { HttpService } from 'src/app/services/http/http.service';
import { PrivateService } from '../../private.service';

@Injectable({
	providedIn: 'root'
})
export class CreateEndpointService {
	constructor(private privateService: PrivateService, private http: HttpService) {}

	addNewEndpoint(requestDetails: { body: any; token?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: requestDetails.token ? '/endpoints' : this.privateService.urlFactory('org_project') + `/endpoints`,
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

	editEndpoint(requestDetails: { endpointId: string; body: any; token?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${requestDetails.token ? '' : this.privateService.urlFactory('org_project')}/endpoints/${requestDetails.endpointId}`,
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

	getEndpoint(endpointId: string, token?: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: token ? '/endpoints' : this.privateService.urlFactory('org_project') + `/endpoints/${endpointId}`,
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
