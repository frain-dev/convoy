import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class AddAnalyticsService {
	constructor(private http: HttpService) {}

	addAnalytics(requestDetails: { is_analytics_enabled: boolean }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/configuration`,
					method: 'post',
					body: requestDetails
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
