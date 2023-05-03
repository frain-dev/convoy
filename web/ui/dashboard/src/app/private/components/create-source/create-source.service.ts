import { Injectable } from '@angular/core';
import { SOURCE } from 'src/app/models/source.model';
import { HTTP_RESPONSE } from 'src/app/models/global.model';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class CreateSourceService {
	constructor(private http: HttpService) {}

	createSource(requestData: { sourceData: SOURCE }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const sourceResponse = await this.http.request({
					url: `/sources`,
					method: 'post',
					body: requestData.sourceData,
					level: 'org_project'
				});

				return resolve(sourceResponse);
			} catch (error) {
				return reject(error);
			}
		});
	}

	updateSource(requestDetails: { data: any; id: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const projectResponse = await this.http.request({
					url: `/sources/${requestDetails.id}`,
					method: 'put',
					body: requestDetails.data,
					level: 'org_project'
				});

				return resolve(projectResponse);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getSourceDetails(sourceId: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const projectResponse = await this.http.request({
					url: `/sources/${sourceId}`,
					method: 'get',
					level: 'org_project'
				});

				return resolve(projectResponse);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
