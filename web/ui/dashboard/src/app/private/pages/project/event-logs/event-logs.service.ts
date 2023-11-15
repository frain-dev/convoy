import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/global.model';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class EventLogsService {
	constructor(private http: HttpService) {}

	getRetryCount(requestDetails: { endpointId?: string; page?: number; startDate?: string; endDate?: string; sourceId?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/events/countbatchreplayevents`,
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

	retryEvent(requestDetails: { eventId: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/events/${requestDetails.eventId}/replay`,
					method: 'put',
					level: 'org_project'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	batchRetryEvent(requestDetails: { page: number; startDate: string; endDate: string; endpointId?: string; sourceId?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/events/batchreplay`,
					method: 'post',
					body: null,
					level: 'org_project',
					query: requestDetails
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
