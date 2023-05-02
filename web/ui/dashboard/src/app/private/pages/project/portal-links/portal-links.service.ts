import { Injectable } from '@angular/core';
import { CURSOR } from 'src/app/models/global.model';
import { HTTP_RESPONSE } from 'src/app/models/global.model';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class PortalLinksService {
	constructor(private http: HttpService) {}

	getPortalLinks(requestDetails: CURSOR & { endpointId?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				if (!requestDetails?.next_page_cursor && !requestDetails?.prev_page_cursor) requestDetails = { next_page_cursor: String(Number.MAX_SAFE_INTEGER), direction: 'next', endpointId: requestDetails?.endpointId };

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
