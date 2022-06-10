import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'convoy-app/lib/models/http.model';
import { SOURCE } from 'src/app/models/group.model';
import { HttpService } from 'src/app/services/http/http.service';
import { PrivateService } from '../../private.service';

@Injectable({
	providedIn: 'root'
})
export class CreateSourceService {
	projectId: string = this.privateService.projectId;

	constructor(private http: HttpService, private privateService: PrivateService) {}

	getOrgId() {
		return localStorage.getItem('ORG_ID');
	}

	createSource(requestData: { sourceData: SOURCE }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const sourceResponse = await this.http.request({
					url: `/organisations/${this.getOrgId()}/groups/${this.projectId}/sources`,
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
