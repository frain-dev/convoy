import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class AppService {
	constructor(private http: HttpService) {}

	async getSubscriptions(token: string): Promise<HTTP_RESPONSE> {
		try {
			const response = await this.http.request({ url: '/subscriptions', method: 'get', token });
			return response;
		} catch (error: any) {
			return error;
		}
	}

	async deleteSubscription(token: string, subscriptionId: string): Promise<HTTP_RESPONSE> {
		try {
			const response = await this.http.request({ url: `/subscriptions/${subscriptionId}`, method: 'delete', token });
			return response;
		} catch (error: any) {
			return error;
		}
	}
}
