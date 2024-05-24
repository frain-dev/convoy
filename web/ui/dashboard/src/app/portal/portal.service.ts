import { Injectable } from '@angular/core';
import { HttpService } from '../services/http/http.service';
import { HTTP_RESPONSE } from '../models/global.model';

@Injectable({
	providedIn: 'root'
})
export class PortalService {
	constructor(private http: HttpService) {}

	getSubscriptions(): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({ url: `/subscriptions`, method: 'get' });
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getPortalDetail(): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({ url: `/portal_link`, method: 'get' });
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	deleteSubscription(subscriptionId: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({ url: `/subscriptions/${subscriptionId}`, method: 'delete' });
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
