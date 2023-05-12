import { Injectable } from '@angular/core';
import { CURSOR, HTTP_RESPONSE } from 'src/app/models/global.model';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class MetaEventsService {
	constructor(private http: HttpService) {}

	getMetaEvents(requestDetails?: CURSOR): Promise<HTTP_RESPONSE> {
		if (!requestDetails) requestDetails = { next_page_cursor: String(Number.MAX_SAFE_INTEGER), direction: 'next' };

		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/meta-events`,
					method: 'get',
					query: requestDetails,
					level: 'org_project'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	retryMetaEvent(eventId: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/meta-events/${eventId}/resend`,
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
