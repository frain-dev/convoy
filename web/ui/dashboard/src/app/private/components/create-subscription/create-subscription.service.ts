import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { HttpService } from 'src/app/services/http/http.service';
import { PrivateService } from '../../private.service';

@Injectable({
	providedIn: 'root'
})
export class CreateSubscriptionService {
	constructor(private http: HttpService, private privateService: PrivateService) {}

	createSubscription(requestDetails: any, token?: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const projectResponse = await this.http.request({
					url: `${token ? '' : this.privateService.urlFactory('org_project')}/subscriptions`,
					method: 'post',
					body: requestDetails,
					token
				});

				return resolve(projectResponse);
			} catch (error) {
				return reject(error);
			}
		});
	}

	updateSubscription(requestDetails: { data: any; id: string; token?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const projectResponse = await this.http.request({
					url: `${requestDetails.token ? '' : this.privateService.urlFactory('org_project')}/subscriptions/${requestDetails.id}`,
					method: 'put',
					body: requestDetails.data,
					token: requestDetails.token
				});

				return resolve(projectResponse);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getSubscriptionDetail(subscriptionId: string, token?: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const projectResponse = await this.http.request({
					url: `${token ? '' : this.privateService.urlFactory('org_project')}/subscriptions/${subscriptionId}`,
					method: 'get',
					token
				});

				return resolve(projectResponse);
			} catch (error) {
				return reject(error);
			}
		});
	}

	async getAppPortalApp(token: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/apps`,
					method: 'get',
					token
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

    testSubsriptionFilter(requestDetails: { schema: any; request: any }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const projectResponse = await this.http.request({
					url: `${this.privateService.urlFactory('org_project')}/subscriptions/test_filter`,
					method: 'post',
					body: requestDetails
				});

				return resolve(projectResponse);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
