import { Injectable } from '@angular/core';
import { SOURCE } from 'src/app/models/group.model';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { HttpService } from 'src/app/services/http/http.service';
import { PrivateService } from '../../private.service';

@Injectable({
	providedIn: 'root'
})
export class CreateSourceService {
	constructor(private http: HttpService, private privateService: PrivateService) {}

	createSource(requestData: { sourceData: SOURCE }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const sourceResponse = await this.http.request({
					url: `${this.privateService.urlFactory('org_project')}/sources`,
					method: 'post',
					body: requestData.sourceData
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
					url: `${this.privateService.urlFactory('org_project')}/sources/${requestDetails.id}`,
					method: 'put',
					body: requestDetails.data
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
					url: `${this.privateService.urlFactory('org_project')}/sources/${sourceId}`,
					method: 'get'
				});

				return resolve(projectResponse);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
