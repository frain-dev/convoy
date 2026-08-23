import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/global.model';
import { HttpService } from 'src/app/services/http/http.service';
import { GeneralService } from 'src/app/services/general/general.service';

@Injectable({
	providedIn: 'root'
})
export class EventsService {
	constructor(private http: HttpService, private generalService: GeneralService) {}

	getEvents(requestDetails?: { page?: number; idempotencyKey?: string; startDate?: string; endDate?: string; query?: string; body?: string; sourceId?: string; endpointId?: string; next_page_cursor?: string; prev_page_cursor?: string; direction?: 'next' | 'prev' }): Promise<HTTP_RESPONSE> {
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

	getEventDeliveries(requestDetails?: { page?: any; startDate?: string; endDate?: string; endpointId?: string; eventId?: string; sourceId?: string; status?: any; query?: string; next_page_cursor?: string; sort?: string }): Promise<HTTP_RESPONSE> {
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
					query: requestDetails,
					hideNotification: true
				});

				return resolve(response);
			} catch (error) {
				const raw = typeof error === 'string' ? error : '';
				const timedOut = /took too long|timed out|timeout|504/i.test(raw);
				this.generalService.showNotification({
					message: timedOut ? "Couldn't load the chart in time. Try a shorter date range." : raw || 'Could not load the chart. Try again.',
					style: 'error'
				});
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

	// Exact live count for Retry All. Do not use this for the table label.
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

	// Per-status totals from /eventdeliveries/statustotals (daily rollup once
	// backfill has completed). Display-only: hideNotification, and null on
	// transport/timeout so a failed read cannot render as zero.
	//
	// Do not use this for Retry All. That path must stay on getRetryCount.
	async getStatusTotals(requestDetails: { startDate?: string; endDate?: string; endpointId?: string }): Promise<Record<string, number> | null> {
		try {
			const response = await this.http.request({
				url: `/eventdeliveries/statustotals`,
				method: 'get',
				level: 'org_project',
				query: requestDetails,
				hideNotification: true
			});

			return response?.data?.totals || {};
		} catch (error) {
			return null;
		}
	}

	// Success/failure totals for the summary cards. Shared by the project and
	// portal delivery screens. A missing status on a successful response is 0;
	// a failed request is null so the caller shows a dash.
	async getSummaryDeliveryCounts(requestDetails: { startDate?: string; endDate?: string; endpointId?: string }): Promise<{ success: number | null; failure: number | null }> {
		const totals = await this.getStatusTotals(requestDetails);
		if (!totals) return { success: null, failure: null };
		return { success: totals['Success'] ?? 0, failure: totals['Failure'] ?? 0 };
	}
}
