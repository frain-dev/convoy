import { Injectable } from '@angular/core';
import { HttpService } from '../services/http/http.service';
import { HTTP_RESPONSE } from '../models/global.model';
import { ActivatedRoute } from '@angular/router';

@Injectable({
	providedIn: 'root'
})
export class PortalService {
	ownerId: string = this.route.snapshot.queryParams.owner_id;

	constructor(private http: HttpService, private route: ActivatedRoute) {}

	getSubscriptions(): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const query: any = {};
				if (this.ownerId) query.owner_id = this.ownerId;

				const response = await this.http.request({
					url: `/subscriptions`,
					method: 'get',
					query
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getPortalDetail(): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const query: any = {};
				if (this.ownerId) query.owner_id = this.ownerId;

				const response = await this.http.request({
					url: `/portal_link`,
					method: 'get',
					query
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	deleteSubscription(subscriptionId: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const query: any = {};
				if (this.ownerId) query.owner_id = this.ownerId;

				const response = await this.http.request({
					url: `/subscriptions/${subscriptionId}`,
					method: 'delete',
					query
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}
}