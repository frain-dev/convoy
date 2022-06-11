import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'convoy-app/lib/models/http.model';
import { PrivateService } from 'src/app/private/private.service';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class SubscriptionsService {
	projectId: string = this.privateService.activeProjectDetails.uid;

	constructor(private http: HttpService, private privateService: PrivateService) {}

	getSubscriptions(requestDetails?: { page?: number }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const subscriptionsResponse = await this.http.request({
					url: `${this.privateService.urlFactory('org_project')}/subscriptions?page=${requestDetails?.page || 1}`,
					method: 'get'
				});

				return resolve(subscriptionsResponse);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	deleteSubscription(subscriptionId: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const sourceResponse = await this.http.request({
					url: `${this.privateService.urlFactory('org_project')}/subscriptions/${subscriptionId}`,
					method: 'delete'
				});

				return resolve(sourceResponse);
			} catch (error: any) {
				return reject(error);
			}
		});
	}
}
