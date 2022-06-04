import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { HttpService } from 'src/app/services/http/http.service';
import { ProjectService } from '../../pages/project/project.service';

@Injectable({
	providedIn: 'root'
})
export class CreateAppService {
	projectId: string = this.projectService.activeProject;
	constructor(private http: HttpService, private projectService: ProjectService) {}

	async updateApp(requestDetails: { appId: string; body: any }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/apps/${requestDetails.appId}?groupId=${this.projectId}`,
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
					url: `/apps?groupId=${this.projectId}`,
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
					url: `/apps/${requestDetails.appId}/endpoints?groupId=${this.projectId}`,
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
