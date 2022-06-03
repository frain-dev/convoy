import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { PrivateService } from 'src/app/private/private.service';
import { HttpService } from 'src/app/services/http/http.service';
import { ProjectService } from '../../project.service';

@Injectable({
	providedIn: 'root'
})
export class AppDetailsService {
	projectId: string = this.privateService.activeProjectId;
	constructor(private http: HttpService, private privateService: PrivateService) {}

	async getAppPortalToken(requestDetails: { appId: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/apps/${requestDetails.appId}/keys?groupId=${this.projectId}`,
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
					url: `/apps/${appId}?groupId=${this.projectId}`,
					method: 'get'
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}
}
