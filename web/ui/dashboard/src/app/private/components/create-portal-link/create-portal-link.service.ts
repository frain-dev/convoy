import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class CreatePortalLinkService {
	constructor(private http: HttpService) {}

	createPortalLink(requestDetails: { data: any }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const projectResponse = await this.http.request({
					url: `/portal-links`,
					method: 'post',
					body: requestDetails.data,
					level: 'org_project'
				});

				return resolve(projectResponse);
			} catch (error) {
				return reject(error);
			}
		});
	}

	updatePortalLink(requestDetails: { data: any; linkId: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const projectResponse = await this.http.request({
					url: `/portal-links/${requestDetails.linkId}`,
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

	getPortalLink(linkUid: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/portal-links/${linkUid}`,
					method: 'get',
					level: 'org_project'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
