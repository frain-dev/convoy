import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { PrivateService } from 'src/app/private/private.service';
import { HttpService } from 'src/app/services/http/http.service';
import { ProjectService } from '../project.service';

@Injectable({
	providedIn: 'root'
})
export class EventsService {
	projectId: string = this.privateService.activeProjectId;

	constructor(private http: HttpService, private privateService: PrivateService) {}

	async getEvents(requestDetails: { pageNo: number; startDate: string; endDate: string; appId: string; query?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/events?groupId=${this.projectId}&sort=AESC&page=${requestDetails.pageNo}&perPage=20&startDate=${requestDetails.startDate}&endDate=${requestDetails.endDate}&appId=${requestDetails.appId}&query=${requestDetails?.query}`,
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
					url: `/eventdeliveries?groupId=${this.projectId}&eventId=${requestDetails.eventId}&page=${requestDetails.pageNo}&startDate=${requestDetails.startDate}&endDate=${requestDetails.endDate}&appId=${requestDetails.appId}${requestDetails.statusQuery}`,
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
					url: `/apps?groupId=${this.projectId}&sort=AESC&page=${requestDetails.pageNo}&perPage=20${requestDetails?.searchString ? `&q=${requestDetails?.searchString}` : ''}`,
					method: 'get'
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	async getEventDeliveryAttempts(requestDetails: { eventDeliveryId: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/eventdeliveries/${requestDetails.eventDeliveryId}/deliveryattempts?groupId=${this.projectId}`,
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
					url: `/dashboard/summary?groupId=${this.projectId}&startDate=${requestDetails.startDate}&endDate=${requestDetails.endDate}&type=${requestDetails.frequency}`,
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
					url: `/eventdeliveries/${requestDetails.eventId}/resend?groupId=${this.projectId}`,
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
					url: `/eventdeliveries/forceresend?groupId=${this.projectId}`,
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
					url: `/eventdeliveries/batchretry?groupId=${this.projectId}&eventId=${requestDetails.eventId || ''}&page=${requestDetails.pageNo}&startDate=${requestDetails.startDate}&endDate=${
						requestDetails.endDate
					}&appId=${requestDetails.appId}${requestDetails.statusQuery || ''}`,
					method: 'post',
					body: null
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}
}
