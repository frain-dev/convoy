import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/global.model';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class CreateSubscriptionService {
	constructor(private http: HttpService) {}

	createSubscription(requestDetails: any): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const projectResponse = await this.http.request({
					url: `/subscriptions`,
					method: 'post',
					body: requestDetails,
					level: 'org_project'
				});

				return resolve(projectResponse);
			} catch (error) {
				return reject(error);
			}
		});
	}

	updateSubscription(requestDetails: { data: any; id: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const projectResponse = await this.http.request({
					url: `/subscriptions/${requestDetails.id}`,
					method: 'put',
					body: requestDetails.data,
					level: 'org_project'
				});

				return resolve(projectResponse);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getSubscriptionDetail(subscriptionId: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const projectResponse = await this.http.request({
					url: `/subscriptions/${subscriptionId}`,
					method: 'get',
					level: 'org_project'
				});

				return resolve(projectResponse);
			} catch (error) {
				return reject(error);
			}
		});
	}

	async getPortalProject(): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/project`,
					method: 'get'
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
					url: `/subscriptions/test_filter`,
					method: 'post',
					body: requestDetails,
					level: 'org_project'
				});

				return resolve(projectResponse);
			} catch (error) {
				return reject(error);
			}
		});
	}


    testTransformFunction(requestDetails: { payload: any; function: any }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const projectResponse = await this.http.request({
					url: `/subscriptions/test_function`,
					method: 'post',
					body: requestDetails,
					level: 'org_project'
				});

				return resolve(projectResponse);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
