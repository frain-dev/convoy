import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'convoy-app/lib/models/http.model';
import { HttpService } from 'src/app/services/http/http.service';
import { PrivateService } from '../../private.service';

@Injectable({
	providedIn: 'root'
})
export class CreateSubscriptionService {
	projectId: string = this.privateService.activeProjectDetails.uid;

	constructor(private http: HttpService, private privateService: PrivateService) {}

	createSubscription(requestDetails: any): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const projectResponse = await this.http.request({
					url: `${this.privateService.urlFactory('org_project')}/subscriptions`,
					method: 'post',
					body: requestDetails
				});

				return resolve(projectResponse);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	updateSubscription(requestDetails: { data: any; id: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const projectResponse = await this.http.request({
					url: `${this.privateService.urlFactory('org_project')}/subscriptions/${requestDetails.id}`,
					method: 'put',
					body: requestDetails
				});

				return resolve(projectResponse);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	getSubscriptionDetail(subscriptionId: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const projectResponse = await this.http.request({
					url: `${this.privateService.urlFactory('org_project')}/subscriptions/${subscriptionId}`,
					method: 'get'
				});

				return resolve(projectResponse);
			} catch (error: any) {
				return reject(error);
			}
		});
	}
}
