import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { HttpService } from 'src/app/services/http/http.service';
import { ProjectService } from '../project.service';

@Injectable({
	providedIn: 'root'
})
export class SourcesService {
	projectId: string = this.projectService.activeProject;

	constructor(private http: HttpService, private projectService: ProjectService) {}

	getSources(requestDetails?: { page?: number }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const sourcesResponse = await this.http.request({
					url: `/sources?groupId=${this.projectId}&page=${requestDetails?.page}`,
					method: 'get'
				});

				return resolve(sourcesResponse);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	deleteSource(sourceId: string | undefined): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const sourceResponse = await this.http.request({
					url: `/sources/${sourceId}?groupId=${this.projectId}`,
					method: 'delete'
				});

				return resolve(sourceResponse);
			} catch (error: any) {
				return reject(error);
			}
		});
	}
}
