import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { PrivateService } from 'src/app/private/private.service';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class EventLogsService {
	constructor(private privateService: PrivateService, private http: HttpService) {}

	getEvents(requestDetails: { pageNo: number; startDate: string; endDate: string; endpointId: string; query?: string; token?: string; sourceId?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${requestDetails.token ? '' : this.privateService.urlFactory('org_project')}/events?sort=AESC&page=${requestDetails.pageNo}&perPage=20&startDate=${requestDetails.startDate}&endDate=${requestDetails.endDate}&endpointId=${
						requestDetails.endpointId
					}&query=${requestDetails?.query || ''}&sourceId=${requestDetails.sourceId || ''}`,
					method: 'get',
					token: requestDetails.token
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getEventDeliveries(requestDetails: { pageNo: number; startDate?: string; endDate?: string; endpointId?: string; eventId: string; statusQuery: string; token?: string; sourceId?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${requestDetails.token ? '' : this.privateService.urlFactory('org_project')}/eventdeliveries?eventId=${requestDetails.eventId}&page=${requestDetails.pageNo}&startDate=${requestDetails.startDate}&endDate=${requestDetails.endDate}&endpointId=${
						requestDetails.endpointId
					}${requestDetails.statusQuery}&sourceId=${requestDetails.sourceId || ''}`,
					method: 'get',
					token: requestDetails.token
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getRetryCount(requestDetails: { endpointId: string; pageNo: number; startDate: string; endDate: string; sourceId?: string; token?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${requestDetails.token ? '' : this.privateService.urlFactory('org_project')}/events/countbatchreplayevents?page=${requestDetails.pageNo}&startDate=${requestDetails.startDate}&endDate=${requestDetails.endDate}&endpointId=${requestDetails.endpointId}${
						requestDetails.sourceId ? '&sourceId=' + requestDetails.sourceId : ''
					}${requestDetails.token ? '&token=' + requestDetails.token : ''}`,
					method: 'get',
					token: requestDetails.token
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	retryEvent(requestDetails: { eventId: string; token?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${this.privateService.urlFactory('org_project')}/events/${requestDetails.eventId}/replay`,
					method: 'put',
					token: requestDetails.token
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	batchRetryEvent(requestDetails: { pageNo: number; startDate: string; endDate: string; endpointId: string; sourceId?: string; token?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${requestDetails.token ? '' : this.privateService.urlFactory('org_project')}/events/batchreplay?page=${requestDetails.pageNo}&startDate=${requestDetails.startDate}&endDate=${requestDetails.endDate}&endpointId=${requestDetails.endpointId}${
						requestDetails.sourceId ? '&sourceId=' + requestDetails.sourceId : ''
					}${requestDetails.token ? '&token=' + requestDetails.token : ''}`,
					method: 'post',
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
