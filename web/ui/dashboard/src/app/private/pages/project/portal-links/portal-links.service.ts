import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class PortalLinksService {
	constructor(private http: HttpService) {}

	getPortalLinks(requestDetails: { page: number; q?: string; endpointId?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/portal-links`,
					method: 'get',
					level: 'org_project',
					query: requestDetails
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	revokePortalLink(requestDetails: { linkId: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/portal-links/${requestDetails.linkId}/revoke`,
					method: 'put',
					body: null,
					level: 'org_project'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
