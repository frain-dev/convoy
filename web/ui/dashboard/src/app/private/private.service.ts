import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class PrivateService {
	projectId: string = '';

	constructor(private http: HttpService) {}

	getOrgId() {
		return localStorage.getItem('ORG_ID');
	}

	async getApps(requestDetails?: { pageNo?: number; searchString?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/organisations/${this.getOrgId()}/groups/${this.projectId}/apps?sort=AESC&page=${requestDetails?.pageNo || 1}&perPage=20${
						requestDetails?.searchString ? `&q=${requestDetails?.searchString}` : ''
					}`,
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
					url: `/organisations/${this.getOrgId()}/groups/${this.projectId}/sources?page=${requestDetails?.page}`,
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
					url: `/organisations/${this.getOrgId()}/groups/${this.projectId}`,
					method: 'get'
				});

				return resolve(projectResponse);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	async getOrganizations(): Promise<HTTP_RESPONSE> {
		try {
			const response = await this.http.request({
				url: `/organisations`,
				method: 'get'
			});
			return response;
		} catch (error: any) {
			return error;
		}
	}

	async logout(): Promise<HTTP_RESPONSE> {
		try {
			const response = await this.http.request({
				url: '/auth/logout',
				method: 'post',
				body: null
			});
			return response;
		} catch (error: any) {
			return error;
		}
	}
}
