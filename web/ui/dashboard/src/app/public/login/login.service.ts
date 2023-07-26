import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/global.model';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class LoginService {
	constructor(private http: HttpService) {}

	login(requestDetails: { email?: string; password?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: '/auth/login',
					body: requestDetails,
					method: 'post'
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getSignupConfig(): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: '/configuration/is_signup_enabled',
					method: 'get'
				});

				localStorage.setItem('IS_SIGNUP_ENABLED', JSON.stringify(response.data));
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	get signupConfig() {
		const isSignupEnabled = localStorage.getItem('IS_SIGNUP_ENABLED');
		if (isSignupEnabled) return JSON.parse(isSignupEnabled);

		return this.getSignupConfig().then(response => {
			return response.data;
		});
	}
}
