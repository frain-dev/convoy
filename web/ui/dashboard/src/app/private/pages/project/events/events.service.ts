import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class EventsService {
	constructor(private http: HttpService) {}

	async getEventDeliveries(requestDetails: { pageNo: number; activeProjectId: string; startDate?: string; endDate?: string; appId?: string; eventId: string; statusQuery: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/eventdeliveries?groupId=${requestDetails.activeProjectId}&eventId=${requestDetails.eventId}&page=${requestDetails.pageNo}&startDate=${requestDetails.startDate}&endDate=${requestDetails.endDate}&appId=${requestDetails.appId}${requestDetails.statusQuery}`,
					method: 'get'
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	async getApps(requestDetails: { activeProjectId: string; pageNo: number; searchString?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/apps?groupId=${requestDetails.activeProjectId}&sort=AESC&page=${requestDetails.pageNo}&perPage=20${requestDetails?.searchString ? `&q=${requestDetails?.searchString}` : ''}`,
					method: 'get'
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	async getEventDeliveryAttempts(requestDetails: { activeProjectId: string; eventDeliveryId: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/eventdeliveries/${requestDetails.eventDeliveryId}/deliveryattempts?groupId=${requestDetails.activeProjectId}`,
					method: 'get'
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}
}
