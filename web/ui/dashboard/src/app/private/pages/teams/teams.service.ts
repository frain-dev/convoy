import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class TeamsService {
	constructor(private http: HttpService) {}

	async getTeamMembers(requestDetails: { org_id: string }): Promise<HTTP_RESPONSE> {
		try {
			const response = await this.http.request({
				url: `/organisations/${requestDetails.org_id}/members`,
				method: 'get'
			});
			return response;
		} catch (error: any) {
			return error;
		}
	}

	async inviteUserToOrganisation(requestDetails: { firstname: string; lastname: string; email: string; role: string; groups: string[] }): Promise<HTTP_RESPONSE> {
		try {
			const response = await this.http.request({
				url: `/organizations/invite_user`,
				body: requestDetails,
				method: 'post'
			});
			return response;
		} catch (error: any) {
			return error;
		}
	}

	async searchTeamMembers(requestOptions: { query: string }): Promise<HTTP_RESPONSE> {
		try {
			const response = await this.http.request({
				url: `/organizations/members/search${requestOptions.query}`,
				method: 'get'
			});
			return response;
		} catch (error: any) {
			return error;
		}
	}

	async deactivateTeamMember(requestOptions: { memberId: string }) {
		try {
			const response = await this.http.request({
				url: `/organizations/members/${requestOptions.memberId}`,
				method: 'delete'
			});
			return response;
		} catch (error: any) {
			return error;
		}
	}

	async getProjects(): Promise<HTTP_RESPONSE> {
		try {
			const response = await this.http.request({
				url: `/groups`,
				method: 'get'
			});
			return response;
		} catch (error: any) {
			return error;
		}
	}
}
