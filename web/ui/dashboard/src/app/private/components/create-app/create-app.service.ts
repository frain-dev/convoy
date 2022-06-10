import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { HttpService } from 'src/app/services/http/http.service';
import { PrivateService } from '../../private.service';

@Injectable({
	providedIn: 'root'
})
export class CreateAppService {
	projectId: string = this.privateService.projectId;
	constructor(private http: HttpService, private privateService: PrivateService) {}

	getOrgId() {
		return localStorage.getItem('ORG_ID');
	}
	
	async updateApp(requestDetails: { appId: string; body: any }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/organisations/${this.getOrgId()}/groups/${this.projectId}/apps/${requestDetails.appId}`,
					method: 'put',
					body: requestDetails.body
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	async createApp(requestDetails: { body: any }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/organisations/${this.getOrgId()}/groups/${this.projectId}/apps`,
					method: 'post',
					body: requestDetails.body
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	async addNewEndpoint(requestDetails: { appId: string; body: any }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/organisations/${this.getOrgId()}/groups/${this.projectId}/apps/${requestDetails.appId}/endpoints`,
					body: requestDetails.body,
					method: 'post'
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}
}
