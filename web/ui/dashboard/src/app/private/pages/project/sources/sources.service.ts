import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { PrivateService } from 'src/app/private/private.service';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class SourcesService {
	constructor(private http: HttpService, private privateService: PrivateService) {}

	deleteSource(sourceId: string | undefined): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const sourceResponse = await this.http.request({
					url: `${this.privateService.urlFactory('org_project')}/sources/${sourceId}`,
					method: 'delete'
				});

				return resolve(sourceResponse);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
