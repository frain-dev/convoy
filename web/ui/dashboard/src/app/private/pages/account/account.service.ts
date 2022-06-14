import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class AccountService {
	constructor(private http: HttpService) {}

	async logout(): Promise<HTTP_RESPONSE> {
		try {
			const response = await this.http.request({
				url: '/auth/logout',
				method: 'post',
				body: null
			});
			return response;
		} catch (error: any) {
			return error;
		}
	}

	async getUserDetails(requestDetails: { userId: string }): Promise<HTTP_RESPONSE> {
		try {
			const response = await this.http.request({
				url: `/users/${requestDetails.userId}/profile`,
				method: 'get'
			});
			return response;
		} catch (error: any) {
			return error;
		}
	}

	async editBasicInfo(requestDetails: { userId: string; body: { first_name: string; last_name: string; email: string } }): Promise<HTTP_RESPONSE> {
		try {
			const response = await this.http.request({
				url: `/users/${requestDetails.userId}/profile`,
				body: requestDetails.body,
				method: 'put'
			});
			return response;
		} catch (error: any) {
			return error;
		}
	}
	
	async changePassword(requestDetails: { userId: string; body: { current_password: string; password: string; password_confirmation: string } }) {
		try {
			const response = await this.http.request({
				url: `/users/${requestDetails.userId}/password`,
				body: requestDetails.body,
				method: 'put'
			});
			return response;
		} catch (error: any) {
			return error;
		}
	}
}
