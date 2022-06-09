import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class AddTeamMemberService {
	constructor(private http: HttpService) {}

	async getProjects(requestDetails: { pageNo: number; searchString?: string }): Promise<HTTP_RESPONSE> {
		try {
			const response = await this.http.request({
				url: `/groups?page=${requestDetails.pageNo}&perPage=20${requestDetails?.searchString ? `&q=${requestDetails?.searchString}` : ''}`,
				method: 'get'
			});
			return response;
		} catch (error: any) {
			return error;
		}
	}

	async inviteUserToOrganisation(requestDetails: { org_id: string; body: { invitee_email: string; role: { groups: string[]; type: string } } }): Promise<HTTP_RESPONSE> {
		try {
			const response = await this.http.request({
				url: `/organisations/${requestDetails.org_id}/invite_user`,
				body: requestDetails.body,
				method: 'post'
			});
			return response;
		} catch (error: any) {
			return error;
		}
	}
}
