import { Injectable } from '@angular/core';
import { HttpClient, HttpHeaders } from '@angular/common/http';
import { HTTP_RESPONSE } from './models/global.model';
import { BehaviorSubject } from 'rxjs';

@Injectable({
	providedIn: 'root'
})
export class ConvoyDashboardService {
	token: string = '';
	authType!: 'Bearer' | 'Basic';
	url: string = '';
	activeGroupId: string = '';

	alertStatus: BehaviorSubject<{ message: string; style: string; show: boolean }> = new BehaviorSubject<{ message: string; style: string; show: boolean }>({ message: 'testing', style: 'info', show: false });

	constructor(private httpClient: HttpClient) {}

	request(requestDetails: { url: string; body?: any; method: 'get' | 'post' | 'delete' | 'put' }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const requestHeader = new HttpHeaders({
					Authorization: `${this.authType} ${this.token}`
				});
				const requestResponse: any = await this.httpClient.request(requestDetails.method, this.url + requestDetails.url, { headers: requestHeader, body: requestDetails.body }).toPromise();
				return resolve(requestResponse);
			} catch (error) {
				return reject(error);
			}
		});
	}

	showNotification(details: { message: string; style: string }) {
		this.alertStatus.next({ message: details.message, style: details.style, show: true });
		setTimeout(() => {
			this.dismissNotification();
		}, 4000);
	}

	dismissNotification() {
		this.alertStatus.next({ message: '', style: '', show: false });
	}

	async getGroups(): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const groupsResponse = await this.request({
					url: '/groups',
					method: 'get'
				});

				this.activeGroupId = groupsResponse.data[0].uid;
				return resolve(groupsResponse);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	async getEvents(requestDetails: { pageNo: number; startDate: string; endDate: string; appId: string; query?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.request({
					url: `/events?groupId=${this.activeGroupId}&sort=AESC&page=${requestDetails.pageNo}&perPage=20&startDate=${requestDetails.startDate}&endDate=${requestDetails.endDate}&appId=${requestDetails.appId}&query=${requestDetails?.query}`,
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
				const response = await this.request({
					url: `/eventdeliveries?groupId=${this.activeGroupId}&eventId=${requestDetails.eventId}&page=${requestDetails.pageNo}&startDate=${requestDetails.startDate}&endDate=${requestDetails.endDate}&appId=${requestDetails.appId}${requestDetails.statusQuery}`,
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
				const response = await this.request({
					url: `/eventdeliveries/${requestDetails.eventDeliveryId}/deliveryattempts?groupId=${this.activeGroupId}`,
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
				const response = await this.request({
					url: `/apps?groupId=${this.activeGroupId}&sort=AESC&page=${requestDetails.pageNo}&perPage=20${requestDetails?.searchString ? `&q=${requestDetails?.searchString}` : ''}`,
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
				const response = await this.request({
					url: `/eventdeliveries/${requestDetails.eventId}/resend?groupId=${this.activeGroupId}`,
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
				const response = await this.request({
					url: `/eventdeliveries/forceresend?groupId=${this.activeGroupId}`,
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
				const response = await this.request({
					url: `/eventdeliveries/batchretry?groupId=${this.activeGroupId}&eventId=${requestDetails.eventId || ''}&page=${requestDetails.pageNo}&startDate=${requestDetails.startDate}&endDate=${
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

	async updateApp(requestDetails: { appId: string; body: any }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.request({
					url: `/apps/${requestDetails.appId}?groupId=${this.activeGroupId}`,
					method: 'put',
					body: requestDetails.body
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	async createApp(requestDetails: { body: any }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.request({
					url: `/apps?groupId=${this.activeGroupId}`,
					method: 'post',
					body: requestDetails.body
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	async deleteApp(requestDetails: { appId: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.request({
					url: `/apps/${requestDetails.appId}?groupId=${this.activeGroupId}`,
					method: 'delete'
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	async addNewEndpoint(requestDetails: { appId: string; body: any }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.request({
					url: `/apps/${requestDetails.appId}/endpoints?groupId=${this.activeGroupId}`,
					body: requestDetails.body,
					method: 'post'
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	async sendEvent(requestDetails: { body: any }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.request({
					url: `/events?groupId=${this.activeGroupId}`,
					body: requestDetails.body,
					method: 'post'
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
				const response = await this.request({
					url: `/eventdeliveries/countbatchretryevents?groupId=${this.activeGroupId}&eventId=${requestDetails.eventId}&page=${requestDetails.pageNo}&startDate=${requestDetails.startDate}&endDate=${
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

	async getConfigDetails(): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.request({
					url: `/dashboard/config?groupId=${this.activeGroupId}`,
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
				const response = await this.request({
					url: `/dashboard/summary?groupId=${this.activeGroupId}&startDate=${requestDetails.startDate}&endDate=${requestDetails.endDate}&type=${requestDetails.frequency}`,
					method: 'get'
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	async getAppPortalToken(requestDetails: { appId: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.request({
					url: `/apps/${requestDetails.appId}/keys?groupId=${this.activeGroupId}`,
					method: 'post',
					body: {}
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}
}
