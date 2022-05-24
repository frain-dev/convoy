import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'convoy-app/lib/models/http.model';
import { SOURCE } from 'src/app/models/group.model';
import { HttpService } from 'src/app/services/http/http.service';
import { ProjectService } from '../../pages/project/project.service';

@Injectable({
	providedIn: 'root'
})
export class CreateSourceService {
	projectId: string = this.projectService.activeProject;

	constructor(private http: HttpService, private projectService: ProjectService) {}

	createSource(requestData: { sourceData: SOURCE }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const sourceResponse = await this.http.request({
					url: `/sources?groupId=${this.projectId}`,
					method: 'post',
					body: requestData.sourceData
				});

				return resolve(sourceResponse);
			} catch (error: any) {
				return reject(error);
			}
		});
	}
}
