import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/global.model';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class ResetPasswordService {
	constructor(private http: HttpService) {}

	resetPassword(requestDetails: { token: string; body: { email: string; password: string; password_confirmation: string } }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/users/reset-password?token=${requestDetails.token}`,
					body: requestDetails.body,
					method: 'post'
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
