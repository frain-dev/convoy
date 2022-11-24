import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { HttpService } from 'src/app/services/http/http.service';
import { PrivateService } from '../../private.service';

@Injectable({
	providedIn: 'root'
})
export class CreatePortalLinkService {
	constructor(private privateService: PrivateService, private http: HttpService) {}

	createPortalLink(requestDetails: { data: any; token?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const projectResponse = await this.http.request({
					url: `${requestDetails.token ? '' : this.privateService.urlFactory('org_project')}/portal-links`,
					method: 'post',
					body: requestDetails.data,
					token: requestDetails.token
				});

				return resolve(projectResponse);
			} catch (error) {
				return reject(error);
			}
		});
	}

	updatePortalLink(requestDetails: { data: any; linkId: string; token?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const projectResponse = await this.http.request({
					url: `${requestDetails.token ? '' : this.privateService.urlFactory('org_project')}/portal-links/${requestDetails.linkId}`,
					method: 'put',
					body: requestDetails.data,
					token: requestDetails.token
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
					url: `${this.privateService.urlFactory('org_project')}/portal-links/${linkUid}`,
					method: 'get'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
