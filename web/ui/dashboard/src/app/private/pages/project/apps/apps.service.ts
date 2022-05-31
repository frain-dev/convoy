import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { HttpService } from 'src/app/services/http/http.service';
import { ProjectService } from '../project.service';

@Injectable({
	providedIn: 'root'
})
export class AppsService {
	projectId: string = this.projectService.activeProject;

	constructor(private http: HttpService, private projectService: ProjectService) {}

	async getApps(requestDetails: { pageNo: number; searchString?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/apps?groupId=${this.projectId}&sort=AESC&page=${requestDetails.pageNo}&perPage=20${requestDetails?.searchString ? `&q=${requestDetails?.searchString}` : ''}`,
					method: 'get'
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}
}
