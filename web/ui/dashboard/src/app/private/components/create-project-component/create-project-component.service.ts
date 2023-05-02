import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/global.model';
import { HttpService } from 'src/app/services/http/http.service';
import { ProjectService } from '../../pages/project/project.service';

@Injectable({
	providedIn: 'root'
})
export class CreateProjectComponentService {
	constructor(private http: HttpService, private projectService: ProjectService) {}

	createProject(requestDetails: { name: string; strategy: { duration: string; retry_count: string; type: string }; signature: { header: string; hash: string }; disable_endpoint: boolean; rate_limit: number; rate_limit_duration: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/projects`,
					body: requestDetails,
					method: 'post',
					level: 'org'
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	updateProject(requestDetails: { name: string; strategy: { duration: string; retry_count: string; type: string }; signature: { header: string; hash: string }; disable_endpoint: boolean; rate_limit: number; rate_limit_duration: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/projects/${this.projectService.activeProjectDetails?.uid}`,
					body: requestDetails,
					method: 'put',
					level: 'org'
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	regenerateKey(): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/projects/${this.projectService.activeProjectDetails?.uid}/security/keys/regenerate`,
					method: 'put',
					body: null,
					level: 'org'
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
