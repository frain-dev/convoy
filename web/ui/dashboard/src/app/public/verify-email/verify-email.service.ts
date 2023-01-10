import { Injectable } from '@angular/core';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class VerifyEmailService {
	constructor(private http: HttpService) {}

	verifyEmail(token: string) {
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
}
