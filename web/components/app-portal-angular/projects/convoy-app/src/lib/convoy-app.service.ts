import { Injectable } from '@angular/core';
import { HttpClient, HttpHeaders } from '@angular/common/http';
import { HTTP_RESPONSE } from './models/global.model';
import { BehaviorSubject } from 'rxjs';

@Injectable({
	providedIn: 'root'
})
export class ConvoyAppService {
	apiURL!: string;
	token!: string;
	alertStatus: BehaviorSubject<{ message: string; style: string; show: boolean }> = new BehaviorSubject<{ message: string; style: string; show: boolean }>({ message: 'testing', style: 'info', show: false });

	constructor(private httpClient: HttpClient) {}

	request(requestDetails: { url: string; body?: any; method: 'get' | 'post' | 'delete' | 'put'; token: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const requestHeader = new HttpHeaders({
					Authorization: `Bearer ${requestDetails.token}`
				});
				const requestResponse: any = await this.httpClient
					.request(requestDetails.method, requestDetails.url, {
						headers: requestHeader,
						body: requestDetails.body
					})
					.toPromise();
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

	getAPIURL(endPointUrl: string) {
		const url = '/portal' + endPointUrl;
		return !this.apiURL || this.apiURL === '' ? location.origin + url : this.apiURL + url;
	}

	async getEvents(requestDetails: { appId: string; pageNo: number; startDate: string; endDate: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.request({
					url: this.getAPIURL(`/events?appId=${requestDetails.appId}&sort=AESC&page=${requestDetails.pageNo}&startDate=${requestDetails.startDate}&endDate=${requestDetails.endDate}`),
					method: 'get',
					token: this.token
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	async getEventDeliveries(requestDetails: { appId: string; eventId: string; pageNo: number; startDate: string; endDate: string; statusQuery: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.request({
					url: this.getAPIURL(
						`/eventdeliveries?appId=${requestDetails.appId}&eventId=${requestDetails.eventId}&page=${requestDetails.pageNo}&startDate=${requestDetails.startDate}&endDate=${requestDetails.endDate}&status=${requestDetails.statusQuery}`
					),
					method: 'get',
					token: this.token
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	async getAppDetails(): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.request({
					url: this.getAPIURL(`/apps`),
					method: 'get',
					token: this.token
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	async addNewEndpoint(requestDetails: { body: object }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.request({
					url: this.getAPIURL(`/apps/endpoints`),
					method: 'post',
					body: requestDetails.body,
					token: this.token
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	async getDeliveryAttempts(requestDetails: { eventDeliveryId: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.request({
					url: this.getAPIURL(`/eventdeliveries/${requestDetails.eventDeliveryId}/deliveryattempts`),
					method: 'get',
					token: this.token
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	async fetchRetryCount(requestDetails: { eventId: string; pageNo: number; startDate: string; endDate: string; statusQuery: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.request({
					url: this.getAPIURL(
						`/eventdeliveries/countbatchretryevents?eventId=${requestDetails.eventId}&page=${requestDetails.pageNo}&startDate=${requestDetails.startDate}&endDate=${requestDetails.endDate}${requestDetails.statusQuery}`
					),
					method: 'get',
					token: this.token
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	async batchRetryEvent(requestDetails: { eventId: string; pageNo: number; startDate: string; endDate: string; statusQuery: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.request({
					url: this.getAPIURL(
						`/eventdeliveries/batchretry?eventId=${requestDetails.eventId}&page=${requestDetails.pageNo}&startDate=${requestDetails.startDate}&endDate=${requestDetails.endDate}${requestDetails.statusQuery}`
					),
					method: 'post',
					body: null,
					token: this.token
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	async retryEvent(requestDetails: { eventDeliveryId: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.request({
					url: this.getAPIURL(`/eventdeliveries/${requestDetails.eventDeliveryId}/resend`),
					method: 'put',
					body: null,
					token: this.token
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
					url: this.getAPIURL(`/eventdeliveries/forceresend`),
					method: 'post',
					body: requestDetails.body,
					token: this.token
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}
}
