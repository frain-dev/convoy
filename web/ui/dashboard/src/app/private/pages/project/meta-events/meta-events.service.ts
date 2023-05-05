import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/global.model';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class MetaEventsService {
	constructor(private http: HttpService) {}

	getMetaEvents(requestDetails?: { page?: number; next_page_cursor?: string; prev_page_cursor?: string; direction?: 'next' | 'prev' }): Promise<HTTP_RESPONSE> {
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
