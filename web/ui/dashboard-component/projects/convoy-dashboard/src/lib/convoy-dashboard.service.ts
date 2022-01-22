import { Injectable } from '@angular/core';
import { HttpClient, HttpHeaders } from '@angular/common/http';
import { HTTP_RESPONSE } from './models/http.model';
import { Router } from '@angular/router';

@Injectable({
	providedIn: 'root'
})
export class ConvoyDashboardService {
	constructor(private httpClient: HttpClient, private router: Router) {}

	authDetails() {
		const authDetails = localStorage.getItem('CONVOY_AUTH');
		if (authDetails) {
			const { username, password, managed_service_token } = JSON.parse(authDetails);
			return { token: managed_service_token || btoa(`${username + ':' + password}`), authState: true };
		} else {
			return { authState: false };
		}
	}

	request(requestDetails: { url: string; body?: any; method: 'get' | 'post' | 'delete' | 'put' }): Promise<HTTP_RESPONSE> {
		const token = this.authDetails().token;
		if (!token) {
			this.showNotification({ message: 'You are not logged in' });
			this.router.navigate(['/login']);
		}

		return new Promise(async (resolve, reject) => {
			try {
				const requestHeader = new HttpHeaders({
					Authorization: `Basic ${token}`
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
