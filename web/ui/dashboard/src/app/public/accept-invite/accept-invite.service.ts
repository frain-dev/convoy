import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/global.model';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class AcceptInviteService {
	constructor(private http: HttpService) {}

	acceptInvite(requestDetails: { token: string; body: { first_name: string; last_name: string; password: string; password_confirmation: string; roles: any } }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/organisations/process_invite?accepted=true&token=${requestDetails.token}`,
					body: requestDetails.body,
					method: 'post'
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getUserDetails(invitation_token: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/users/token?token=${invitation_token}`,
					method: 'get'
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
