import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { HttpService } from 'src/app/services/http/http.service';
import { PrivateService } from '../../private.service';

@Injectable({
	providedIn: 'root'
})
export class ProjectsService {
	constructor(private http: HttpService, private privateService: PrivateService) {}

	getProjects(): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const groupsResponse = await this.http.request({
					url: `${this.privateService.urlFactory('org')}/groups`,
					method: 'get'
				});

				return resolve(groupsResponse);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
