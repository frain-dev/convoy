import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { PrivateService } from 'src/app/private/private.service';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class EventsService {
	projectId: string = this.privateService.activeProjectDetails.uid;

	constructor(private http: HttpService, private privateService: PrivateService) {}

	async getEvents(requestDetails: { pageNo: number; startDate: string; endDate: string; appId: string; query?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${this.privateService.urlFactory('org_project')}/events?sort=AESC&page=${requestDetails.pageNo}&perPage=20&startDate=${requestDetails.startDate}&endDate=${
						requestDetails.endDate
					}&appId=${requestDetails.appId}&query=${requestDetails?.query}`,
					method: 'get'
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	async getEventDeliveries(requestDetails: { pageNo: number; startDate?: string; endDate?: string; appId?: string; eventId: string; statusQuery: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${this.privateService.urlFactory('org_project')}/eventdeliveries?eventId=${requestDetails.eventId}&page=${requestDetails.pageNo}&startDate=${requestDetails.startDate}&endDate=${
						requestDetails.endDate
					}&appId=${requestDetails.appId}${requestDetails.statusQuery}`,
					method: 'get'
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	async getApps(requestDetails: { pageNo: number; searchString?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${this.privateService.urlFactory('org_project')}/apps?sort=AESC&page=${requestDetails.pageNo}&perPage=20${requestDetails?.searchString ? `&q=${requestDetails?.searchString}` : ''}`,
					method: 'get'
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	async getEventDeliveryAttempts(requestDetails: { eventDeliveryId: string; eventId: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${this.privateService.urlFactory('org_project')}/eventdeliveries/${requestDetails.eventDeliveryId}/deliveryattempts`,
					method: 'get'
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	async dashboardSummary(requestDetails: { startDate: string; endDate: string; frequency: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${this.privateService.urlFactory('org_project')}/dashboard/summary?startDate=${requestDetails.startDate}&endDate=${requestDetails.endDate}&type=${requestDetails.frequency}`,
					method: 'get'
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	async retryEvent(requestDetails: { eventId: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${this.privateService.urlFactory('org_project')}/eventdeliveries/${requestDetails.eventId}/resend`,
					method: 'put'
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	async forceRetryEvent(requestDetails: { body: object }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${this.privateService.urlFactory('org_project')}/eventdeliveries/forceresend`,
					method: 'post',
					body: requestDetails.body
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	async batchRetryEvent(requestDetails: { eventId: string; pageNo: number; startDate: string; endDate: string; appId: string; statusQuery?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${this.privateService.urlFactory('org_project')}/eventdeliveries/batchretry?eventId=${requestDetails.eventId || ''}&page=${requestDetails.pageNo}&startDate=${
						requestDetails.startDate
					}&endDate=${requestDetails.endDate}&appId=${requestDetails.appId}${requestDetails.statusQuery || ''}`,
					method: 'post',
					body: null
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	async getRetryCount(requestDetails: { appId: string; eventId: string; pageNo: number; startDate: string; endDate: string; statusQuery: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/eventdeliveries/countbatchretryevents?groupId=${this.projectId}&eventId=${requestDetails.eventId}&page=${requestDetails.pageNo}&startDate=${requestDetails.startDate}&endDate=${
						requestDetails.endDate
					}&appId=${requestDetails.appId}${requestDetails.statusQuery || ''}`,
					method: 'get'
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	async getDelivery(eventDeliveryId: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/eventdeliveries/${eventDeliveryId}?groupId=${this.projectId}`,
					method: 'get'
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}
}
