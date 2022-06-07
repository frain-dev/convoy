import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class PrivateService {
	activeProjectId: string = '';

	constructor(private http: HttpService) {}

	async getApps(requestDetails?: { pageNo?: number; searchString?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/apps?groupId=${this.activeProjectId}&sort=AESC&page=${requestDetails?.pageNo || 1}&perPage=20${requestDetails?.searchString ? `&q=${requestDetails?.searchString}` : ''}`,
					method: 'get'
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	getSources(requestDetails?: { page?: number }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const sourcesResponse = await this.http.request({
					url: `/sources?groupId=${this.activeProjectId}&page=${requestDetails?.page}`,
					method: 'get'
				});

				return resolve(sourcesResponse);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	getProjectDetails(): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const projectResponse = await this.http.request({
					url: `/groups/${this.activeProjectId}`,
					method: 'get'
				});

				return resolve(projectResponse);
			} catch (error: any) {
				return reject(error);
			}
		});
	}
}
