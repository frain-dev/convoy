import { Injectable } from '@angular/core';
import { HttpClient, HttpHeaders } from '@angular/common/http';
import { HTTP_RESPONSE } from './models/http.model';
import { BehaviorSubject } from 'rxjs';

@Injectable({
	providedIn: 'root'
})
export class ConvoyAppService {
	alertStatus: BehaviorSubject<{ message: string; style: string; show: boolean }> = new BehaviorSubject<{ message: string; style: string; show: boolean }>({ message: 'testing', style: 'info', show: false });

	constructor(private httpClient: HttpClient) {}

	request(requestDetails: { url: string; body?: any; method: 'get' | 'post' | 'delete' | 'put'; token: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const requestHeader = new HttpHeaders({
					Authorization: `Bearer ${requestDetails.token}`
				});
				const requestResponse: any = await this.httpClient
					.request(requestDetails.method, requestDetails.url, {
						headers: requestHeader,
						body: requestDetails.body
					})
					.toPromise();
				return resolve(requestResponse);
			} catch (error) {
				return reject(error);
			}
		});
	}

	showNotification(details: { message: string; style: string }) {
		this.alertStatus.next({ message: details.message, style: details.style, show: true });
		setTimeout(() => {
			this.dismissNotification();
		}, 4000);
	}

	dismissNotification() {
		this.alertStatus.next({ message: '', style: '', show: false });
	}
	
}
