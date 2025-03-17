import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/global.model';
import { HttpService } from 'src/app/services/http/http.service';
import { FILTER, FILTER_CREATE_REQUEST, FILTER_TEST_REQUEST } from 'src/app/models/filter.model';

@Injectable({
	providedIn: 'root'
})
export class FilterService {
	constructor(private http: HttpService) {}

	createFilter(requestDetails: FILTER_CREATE_REQUEST): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/subscriptions/${requestDetails.subscription_id}/filters`,
					method: 'post',
					body: requestDetails,
					level: 'org_project'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	createFilters(subscriptionId: string, filters: FILTER_CREATE_REQUEST[]): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/subscriptions/${subscriptionId}/filters/bulk`,
					method: 'post',
					body: filters,
					level: 'org_project'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getFilters(subscriptionId: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/subscriptions/${subscriptionId}/filters`,
					method: 'get',
					level: 'org_project'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	updateFilter(subscriptionId: string, filterId: string, requestDetails: Partial<FILTER_CREATE_REQUEST>): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/subscriptions/${subscriptionId}/filters/${filterId}`,
					method: 'put',
					body: requestDetails,
					level: 'org_project'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	bulkUpdateFilters(subscriptionId: string, filters: Array<{ uid: string } & Partial<FILTER_CREATE_REQUEST>>): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				console.log('bulkUpdateFilters called with method: POST, data:', filters);
				const response = await this.http.request({
					url: `/subscriptions/${subscriptionId}/filters/bulk_update`,
					method: 'post',
					body: filters,
					level: 'org_project'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	deleteFilter(subscriptionId: string, filterId: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/subscriptions/${subscriptionId}/filters/${filterId}`,
					method: 'delete',
					level: 'org_project'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	testFilter(requestDetails: FILTER_TEST_REQUEST): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/subscriptions/${requestDetails.subscription_id}/filters/test`,
					method: 'post',
					body: requestDetails,
					level: 'org_project'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
