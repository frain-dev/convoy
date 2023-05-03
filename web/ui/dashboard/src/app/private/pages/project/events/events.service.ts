import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/global.model';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class EventsService {
	constructor(private http: HttpService) {}

	getEvents(requestDetails: { page?: number; startDate: string; endDate: string; query?: string; sourceId?: string; endpointId?: string; next_page_cursor?: string; prev_page_cursor?: string; direction?: 'next' | 'prev' }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/events`,
					method: 'get',
					query: requestDetails,
					level: 'org_project'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getEventDeliveries(requestDetails?: { page?: any; startDate?: string; endDate?: string; endpointId?: string; eventId?: string; sourceId?: string; status?: any; next_page_cursor?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/eventdeliveries`,
					method: 'get',
					query: requestDetails,
					level: 'org_project'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	dashboardSummary(requestDetails: { startDate: string; endDate: string; type: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/dashboard/summary`,
					method: 'get',
					level: 'org_project',
					query: requestDetails
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	retryEvent(requestDetails: { eventId: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/eventdeliveries/${requestDetails.eventId}/resend`,
					method: 'put',
					level: 'org_project'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	forceRetryEvent(requestDetails: { body: object }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/eventdeliveries/forceresend`,
					method: 'post',
					body: requestDetails.body,
					level: 'org_project'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	batchRetryEvent(requestDetails: { eventId?: string; startDate?: string; endDate?: string; endpointId?: string; status?: any }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/eventdeliveries/batchretry`,
					method: 'post',
					body: null,
					level: 'org_project',
					query: requestDetails
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getRetryCount(requestDetails: { endpointId?: string; eventId?: string; startDate?: string; endDate?: string; status?: any }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/eventdeliveries/countbatchretryevents`,
					method: 'get',
					level: 'org_project',
					query: requestDetails
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
