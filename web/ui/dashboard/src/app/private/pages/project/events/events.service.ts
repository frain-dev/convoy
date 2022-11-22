import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { PrivateService } from 'src/app/private/private.service';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class EventsService {
	constructor(private http: HttpService, private privateService: PrivateService) {}

	getEvents(requestDetails: { pageNo: number; startDate: string; endDate: string; appId: string; query?: string; token?: string; sourceId?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${requestDetails.token ? '' : this.privateService.urlFactory('org_project')}/events?sort=AESC&page=${requestDetails.pageNo}&perPage=20&startDate=${requestDetails.startDate}&endDate=${requestDetails.endDate}&appId=${requestDetails.appId}&query=${
						requestDetails?.query || ''
					}&sourceId=${requestDetails.sourceId || ''}`,
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

	getEndpoints(requestDetails: { pageNo: number; searchString?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${this.privateService.urlFactory('org_project')}/endpoints?sort=AESC&page=${requestDetails.pageNo}&perPage=20${requestDetails?.searchString ? `&q=${requestDetails?.searchString}` : ''}`,
					method: 'get'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getEventDeliveryAttempts(requestDetails: { eventDeliveryId: string; token?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${requestDetails.token ? '' : this.privateService.urlFactory('org_project')}/eventdeliveries/${requestDetails.eventDeliveryId}/deliveryattempts`,
					method: 'get',
					token: requestDetails.token
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	dashboardSummary(requestDetails: { startDate: string; endDate: string; frequency: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${this.privateService.urlFactory('org_project')}/dashboard/summary?startDate=${requestDetails.startDate}&endDate=${requestDetails.endDate}&type=${requestDetails.frequency}`,
					method: 'get'
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
					url: `${requestDetails.token ? '' : this.privateService.urlFactory('org_project')}/eventdeliveries/${requestDetails.eventId}/resend`,
					method: 'put',
					token: requestDetails.token
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	forceRetryEvent(requestDetails: { body: object; token?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${requestDetails.token ? '' : this.privateService.urlFactory('org_project')}/eventdeliveries/forceresend`,
					method: 'post',
					body: requestDetails.body,
					token: requestDetails.token
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	batchRetryEvent(requestDetails: { eventId: string; pageNo: number; startDate: string; endDate: string; endpointId: string; statusQuery?: string; token?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${requestDetails.token ? '' : this.privateService.urlFactory('org_project')}/eventdeliveries/batchretry?eventId=${requestDetails.eventId || ''}&page=${requestDetails.pageNo}&startDate=${requestDetails.startDate}&endDate=${requestDetails.endDate}&endpointId=${
						requestDetails.endpointId
					}${requestDetails.statusQuery || ''}`,
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

	getRetryCount(requestDetails: { endpointId: string; eventId: string; pageNo: number; startDate: string; endDate: string; statusQuery: string; token?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${requestDetails.token ? '' : this.privateService.urlFactory('org_project')}/eventdeliveries/countbatchretryevents?eventId=${requestDetails.eventId}&page=${requestDetails.pageNo}&startDate=${requestDetails.startDate}&endDate=${
						requestDetails.endDate
					}&endpointId=${requestDetails.endpointId}${requestDetails.statusQuery || ''}`,
					method: 'get',
					token: requestDetails.token
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getDelivery(eventDeliveryId: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${this.privateService.urlFactory('org_project')}/eventdeliveries/${eventDeliveryId}`,
					method: 'get'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
