import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/global.model';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class TeamsService {
	constructor(private http: HttpService) {}

	getPendingTeamMembers(requestDetails: { page?: number }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/invites/pending`,
					method: 'get',
					level: 'org',
					query: requestDetails
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	resendPendingInvite(inviteID: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/invites/${inviteID}/resend`,
					method: 'post',
					body: null,
					level: 'org'
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	cancelPendingInvite(inviteID: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/invites/${inviteID}/cancel`,
					method: 'post',
					body: null,
					level: 'org'
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	inviteUserToOrganisation(requestDetails: { firstname: string; lastname: string; email: string; role: string; groups: string[] }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/invites`,
					body: requestDetails,
					method: 'post',
					level: 'org'
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	deactivateTeamMember(requestOptions: { memberId: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/members/${requestOptions.memberId}`,
					method: 'delete',
					level: 'org'
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
