import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { HttpService } from 'src/app/services/http/http.service';
import { PrivateService } from '../../private.service';

@Injectable({
	providedIn: 'root'
})
export class CreateProjectComponentService {
	constructor(private http: HttpService, private privateService: PrivateService) {}

	createProject(requestDetails: { name: string; strategy: { duration: string; retry_count: string; type: string }; signature: { header: string; hash: string }; disable_endpoint: boolean; rate_limit: number; rate_limit_duration: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${this.privateService.urlFactory('org')}/projects`,
					body: requestDetails,
					method: 'post'
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	updateProject(requestDetails: { name: string; strategy: { duration: string; retry_count: string; type: string }; signature: { header: string; hash: string }; disable_endpoint: boolean; rate_limit: number; rate_limit_duration: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${this.privateService.urlFactory('org')}/projects/${this.privateService.activeProjectDetails?.uid}`,
					body: requestDetails,
					method: 'put'
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
