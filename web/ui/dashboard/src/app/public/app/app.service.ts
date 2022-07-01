import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class AppService {
	constructor(private http: HttpService) {}

	getSubscriptions(token: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({ url: '/subscriptions', method: 'get', token });
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	deleteSubscription(token: string, subscriptionId: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({ url: `/subscriptions/${subscriptionId}`, method: 'delete', token });
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
