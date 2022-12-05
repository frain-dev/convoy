import { Injectable } from '@angular/core';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class VerifyEmailService {
	constructor(private http: HttpService) {}

	resendVerificationEmail() {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/users/resend_verification_email`,
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
