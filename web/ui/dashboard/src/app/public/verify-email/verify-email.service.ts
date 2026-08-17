import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/global.model';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class VerifyEmailService {
	constructor(private http: HttpService) {}

	verifyEmail(token: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/users/verify_email?token=${token}`,
					body: null,
					method: 'post'
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	resendVerificationEmail(token?: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const query = token ? `?token=${encodeURIComponent(token)}` : '';
				const response = await this.http.request({
					url: `/users/resend_verification_email${query}`,
					body: null,
					method: 'post'
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
