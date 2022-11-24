import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { environment } from 'src/environments/environment';
import axios from 'axios';
import { Router } from '@angular/router';
import { GeneralService } from '../general/general.service';

@Injectable({
	providedIn: 'root'
})
export class HttpService {
	APIURL = `${environment.production ? location.origin : 'http://localhost:5005'}/ui`;
	APP_PORTAL_APIURL = `${environment.production ? location.origin : 'http://localhost:5005'}/portal`;

	constructor(private router: Router, private generalService: GeneralService) {}

	authDetails() {
		const authDetails = localStorage.getItem('CONVOY_AUTH');
		if (authDetails && authDetails !== 'undefined') {
			const { token } = JSON.parse(authDetails);
			return { token: token.access_token, authState: true };
		} else {
			return { authState: false };
		}
	}

	async request(requestDetails: { url: string; body?: any; method: 'get' | 'post' | 'delete' | 'put'; token?: string; hideNotification?: boolean }): Promise<HTTP_RESPONSE> {
		requestDetails.hideNotification = !!requestDetails.hideNotification;
		return new Promise(async (resolve, reject) => {
			try {
				const http = axios.create();

				// Interceptor
				http.interceptors.response.use(
					request => {
						return request;
					},
					error => {
						if (axios.isAxiosError(error)) {
							const errorResponse: any = error.response;
							let errorMessage: any = errorResponse?.data ? errorResponse.data.message : error.message;
							if (error.response?.status == 401 && this.router.url.split('/')[1] !== 'app') {
								// save previous location before session timeout
								if (this.router.url.split('/')[1] !== 'login') localStorage.setItem('CONVOY_LAST_AUTH_LOCATION', location.href);

								// then logout
								this.router.navigate(['/login'], { replaceUrl: true });
								localStorage.removeItem('CONVOY_AUTH');
								this.generalService.showNotification({
									message: errorMessage,
									style: 'error'
								});
								return Promise.reject(error);
							}

							if (!requestDetails.hideNotification) {
								this.generalService.showNotification({
									message: errorMessage,
									style: 'error'
								});
							}
							return Promise.reject(error);
						}

						if (!requestDetails.hideNotification) {
							let errorMessage: string;
							error.error?.message ? (errorMessage = error.error?.message) : (errorMessage = 'An error occured, please try again');
							this.generalService.showNotification({
								message: errorMessage,
								style: 'error'
							});
						}
						return Promise.reject(error);
					}
				);

				const requestHeader = {
					Authorization: requestDetails.token ? '' : `Bearer ${this.authDetails()?.token}`
				};

				// make request
				const { data, status } = await http.request({
					method: requestDetails.method,
					headers: requestHeader,
					url: (requestDetails.token ? this.APP_PORTAL_APIURL : this.APIURL) + requestDetails.url,
					data: requestDetails.body
				});
				resolve(data);
			} catch (error) {
				if (axios.isAxiosError(error)) {
					console.log('error message: ', error.message);
					return reject(error.message);
				} else {
					console.log('unexpected error: ', error);
					return reject('An unexpected error occurred');
				}
			}
		});
	}
}
