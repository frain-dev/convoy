import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { PrivateService } from 'src/app/private/private.service';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class PortalLinksService {
	constructor(private privateService: PrivateService, private http: HttpService) {}

	getPortalLinks(requestDetails: { pageNo: number; searchString?: string; endpointId?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${this.privateService.urlFactory('org_project')}/portal-links?sort=AESC&page=${requestDetails.pageNo}&perPage=20${requestDetails?.searchString ? `&q=${requestDetails?.searchString}` : ''}${
						requestDetails?.endpointId ? `&endpointId=${requestDetails?.endpointId}` : ''
					}`,
					method: 'get'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	revokePortalLink(requestDetails: { linkId: string; token?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: requestDetails.token ? `/portal-links/${requestDetails.linkId}/revoke` : `${this.privateService.urlFactory('org_project')}/portal-links/${requestDetails.linkId}/revoke`,
					method: 'put',
					body: null,
					token: requestDetails.token
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
