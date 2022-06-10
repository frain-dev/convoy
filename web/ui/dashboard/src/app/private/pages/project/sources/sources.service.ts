import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { PrivateService } from 'src/app/private/private.service';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class SourcesService {
	projectId: string = this.privateService.projectId;

	constructor(private http: HttpService, private privateService: PrivateService) {}

	getOrgId() {
		return localStorage.getItem('ORG_ID');
	}

	deleteSource(sourceId: string | undefined): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const sourceResponse = await this.http.request({
					url: `/organisations/${this.getOrgId()}/groups/${this.projectId}/sources/${sourceId}`,
					method: 'delete'
				});

				return resolve(sourceResponse);
			} catch (error: any) {
				return reject(error);
			}
		});
	}
}
