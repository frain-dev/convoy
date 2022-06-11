import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { PrivateService } from 'src/app/private/private.service';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class AppDetailsService {
	projectId: string = this.privateService.activeProjectDetails.uid;

	constructor(private http: HttpService, private privateService: PrivateService) {}

	async getApps(): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${this.privateService.urlFactory('org_project')}/apps?&sort=AESC&page=1&perPage=50`,
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
					url: `${this.privateService.urlFactory('org_project')}/apps/${requestDetails.appId}/keys`,
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
					url: `${this.privateService.urlFactory('org_project')}/apps/${appId}`,
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
					url: `${this.privateService.urlFactory('org_project')}/apps/${requestDetails.appId}/endpoints`,
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
					url: `${this.privateService.urlFactory('org_project')}/events`,
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
