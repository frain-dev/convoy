import { Injectable } from '@angular/core';
import { FLIPT_API_RESPONSE } from 'src/app/models/flipt.model';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class AppService {
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

	async flipt(): Promise<FLIPT_API_RESPONSE> {
		const projectDetails = await this.getProjectDetails();

		return new Promise(async (resolve, reject) => {
			const flagKeys = ['can_create_cli_api_key'];
			const requests: { flagKey: string; entityId: string; context: { group_id: string; organisation_id: string } }[] = [];
			flagKeys.forEach((key: string) =>
				requests.push({
					flagKey: key,
					entityId: key,
					context: {
						group_id: projectDetails.data.uid,
						organisation_id: projectDetails.data.organisation_id
					}
				})
			);

			try {
				const response: any = await this.http.request({ url: `/flags`, method: 'post', body: { requests }, isOut: true, hideNotification: true });
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	async getFlag(flagKey: string): Promise<boolean> {
		try {
			const apiFlagResponse = await this.flipt();

			const flags = apiFlagResponse.responses;
			return !!flags.find(flag => flag.flagKey === flagKey)?.match;
		} catch (error) {
			return false;
		}
	}

	getProjectDetails(): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({ url: `/project`, method: 'get' });
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
