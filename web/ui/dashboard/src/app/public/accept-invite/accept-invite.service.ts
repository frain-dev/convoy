import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class AcceptInviteService {
	constructor(private http: HttpService) {}

	async acceptInvite(requestDetails: { token: string; body: { first_name: string; last_name: string; password: string; password_confirmation: string; roles: any } }): Promise<HTTP_RESPONSE> {
		try {
			const response = await this.http.request({
				url: `/organisations/process_invite?accepted=true&token=${requestDetails.token}`,
				body: requestDetails.body,
				method: 'post'
			});
			return response;
		} catch (error: any) {
			return error;
		}
	}

	async getUserDetails(invitation_token: string): Promise<HTTP_RESPONSE> {
		try {
			const response = await this.http.request({
				url: `/users/token?token=${invitation_token}`,
				method: 'get'
			});
			return response;
		} catch (error: any) {
			return error;
		}
	}
}
