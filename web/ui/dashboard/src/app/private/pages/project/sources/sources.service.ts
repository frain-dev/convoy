import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/global.model';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class SourcesService {
	constructor(private http: HttpService) {}

	deleteSource(sourceId: string | undefined): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const sourceResponse = await this.http.request({
					url: `/sources/${sourceId}`,
					method: 'delete',
					level: 'org_project'
				});

				return resolve(sourceResponse);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
