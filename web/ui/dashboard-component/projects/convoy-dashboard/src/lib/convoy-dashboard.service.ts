import { Injectable } from '@angular/core';
import { HttpClient, HttpHeaders } from '@angular/common/http';
import { HTTP_RESPONSE } from './models/http.model';

@Injectable({
	providedIn: 'root'
})
export class ConvoyDashboardService {
	constructor(private httpClient: HttpClient) {}

	request(requestDetails: { url: string; body?: any; method: 'get' | 'post' | 'delete' | 'put'; token: string; authType: 'Bearer' | 'Basic' }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const requestHeader = new HttpHeaders({
					Authorization: `${requestDetails.authType} ${requestDetails.token}`
				});
				const requestResponse: any = await this.httpClient.request(requestDetails.method, requestDetails.url, { headers: requestHeader, body: requestDetails.body }).toPromise();
				return resolve(requestResponse);
			} catch (error) {
				return reject(error);
			}
		});
	}

	showNotification(details: { message: string }) {
		if (!details.message) return;

		const notificationElement = document.querySelector('.app-notification');
		if (notificationElement) {
			notificationElement.classList.add('show');
			notificationElement.innerHTML = details.message;
		}

		setTimeout(() => {
			notificationElement?.classList.remove('show');
		}, 3000);
	}
}
