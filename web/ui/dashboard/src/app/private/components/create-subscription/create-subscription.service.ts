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
					url: `${token ? '' : this.privateService.urlFactory('org_project')}/subscriptions${token ? '?token=' + token : ''}`,
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
					url: `${requestDetails.token ? '' : this.privateService.urlFactory('org_project')}/subscriptions/${requestDetails.id}${requestDetails.token ? '?token=' + requestDetails.token : ''}`,
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
					url: `${token ? '' : this.privateService.urlFactory('org_project')}/subscriptions/${subscriptionId}${token ? '?token=' + token : ''}`,
					method: 'get',
					token
				});

				return resolve(projectResponse);
			} catch (error) {
				return reject(error);
			}
		});
	}

	async getPortalProject(token: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/project?token=${token}`,
					method: 'get',
					token
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getEndpoints(requestDetails?: { pageNo?: number; searchString?: string; token?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: requestDetails?.token
						? `/endpoints?token=${requestDetails?.token}`
						: this.privateService.urlFactory('org_project') + `/endpoints?sort=AESC&page=${requestDetails?.pageNo || 1}&perPage=20${requestDetails?.searchString ? `&q=${requestDetails?.searchString}` : ''}`,
					method: 'get',
					token: requestDetails?.token
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	testSubsriptionFilter(requestDetails: { schema: any; request: any }, token?: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const projectResponse = await this.http.request({
					url: `${token ? '' : this.privateService.urlFactory('org_project')}/subscriptions/test_filter${token ? `?token=${token}` : ''}`,
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
}
