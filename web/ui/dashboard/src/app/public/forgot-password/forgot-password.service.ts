import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class ForgotPasswordService {
	constructor(private http: HttpService) {}

	async forgotPassword(requestDetails: { email: string }): Promise<HTTP_RESPONSE> {
		try {
			const response = await this.http.request({
				url: '/users/forgot-password',
				body: requestDetails,
				method: 'post'
			});
			return response;
		} catch (error: any) {
			return error;
		}
	}
}
