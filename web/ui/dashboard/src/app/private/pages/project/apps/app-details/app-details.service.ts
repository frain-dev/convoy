import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { PrivateService } from 'src/app/private/private.service';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class AppDetailsService {
	projectId: string = this.privateService.projectId;
	constructor(private http: HttpService, private privateService: PrivateService) {}

	getOrgId() {
		return localStorage.getItem('ORG_ID');
	}

	async getApps(): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/organisations/${this.getOrgId()}/groups/${this.projectId}/apps?sort=AESC&page=1&perPage=50`,
					method: 'get'
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	async getAppPortalToken(requestDetails: { appId: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/organisations/${this.getOrgId()}/groups/${this.projectId}/apps/${requestDetails.appId}/keys`,
					method: 'post',
					body: {}
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	async getApp(appId: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/organisations/${this.getOrgId()}/groups/${this.projectId}/apps/${appId}`,
					method: 'get'
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

	async sendEvent(requestDetails: { body: any }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/organisations/${this.getOrgId()}/groups/${this.projectId}/events`,
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
