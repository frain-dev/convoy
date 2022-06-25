import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { HttpService } from 'src/app/services/http/http.service';
import { PrivateService } from '../../private.service';

@Injectable({
	providedIn: 'root'
})
export class TeamsService {
	constructor(private http: HttpService, private privateService: PrivateService) {}

	async getTeamMembers(requestDetails: { searchString?: string; pageNo?: number }): Promise<HTTP_RESPONSE> {
		try {
			const response = await this.http.request({
				url: `${this.privateService.urlFactory('org')}/members?sort=AESC&page=${requestDetails?.pageNo || 1}&perPage=20${requestDetails?.searchString ? `&q=${requestDetails?.searchString}` : ''}`,
				method: 'get'
			});
			return response;
		} catch (error: any) {
			return error;
		}
	}

	async getPendingTeamMembers(requestDetails: { pageNo?: number }): Promise<HTTP_RESPONSE> {
		try {
			const response = await this.http.request({
				url: `${this.privateService.urlFactory('org')}/invites/pending?sort=AESC&page=${requestDetails?.pageNo || 1}&perPage=20`,
				method: 'get'
			});
			return response;
		} catch (error: any) {
			return error;
		}
	}

	async resendPendingInvite(inviteID: string): Promise<HTTP_RESPONSE> {
		try {
			const response = await this.http.request({
				url: `${this.privateService.urlFactory('org')}/invites/${inviteID}/resend`,
				method: 'post',
				body: null
			});
			return response;
		} catch (error: any) {
			return error;
		}
	}

	async cancelPendingInvite(inviteID: string): Promise<HTTP_RESPONSE> {
		try {
			const response = await this.http.request({
				url: `${this.privateService.urlFactory('org')}/invites/${inviteID}/cancel`,
				method: 'post',
				body: null
			});
			return response;
		} catch (error: any) {
			return error;
		}
	}

	async inviteUserToOrganisation(requestDetails: { firstname: string; lastname: string; email: string; role: string; groups: string[] }): Promise<HTTP_RESPONSE> {
		try {
			const response = await this.http.request({
				url: `${this.privateService.urlFactory('org')}/invites`,
				body: requestDetails,
				method: 'post'
			});
			return response;
		} catch (error: any) {
			return error;
		}
	}

	async deactivateTeamMember(requestOptions: { memberId: string }) {
		try {
			const response = await this.http.request({
				url: `${this.privateService.urlFactory('org')}/members/${requestOptions.memberId}`,
				method: 'delete'
			});
			return response;
		} catch (error: any) {
			return error;
		}
	}
}
