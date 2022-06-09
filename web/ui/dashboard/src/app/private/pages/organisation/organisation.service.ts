import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class OrganisationService {
	constructor(private http: HttpService) {}

	async updateOrganisation(requestDetails: { org_id: string; body: { name: string } }): Promise<HTTP_RESPONSE> {
		try {
			const response = await this.http.request({
				url: `/organisations/${requestDetails.org_id}`,
				method: 'put',
				body: requestDetails.body
			});
			return response;
		} catch (error: any) {
			return error;
		}
	}

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

	async deleteOrganisation(requestDetails: { org_id: string }): Promise<HTTP_RESPONSE> {
		try {
			const response = await this.http.request({
				url: `/organisations/${requestDetails.org_id}`,
				method: 'delete',
				body: null
			});
			return response;
		} catch (error: any) {
			return error;
		}
	}
}
