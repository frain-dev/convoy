import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'convoy-app/lib/models/http.model';
import { HttpService } from 'src/app/services/http/http.service';
import { PrivateService } from '../../private.service';

@Injectable({
	providedIn: 'root'
})
export class CreateProjectComponentService {
	constructor(private http: HttpService, private privateService: PrivateService) {}

	async createProject(requestDetails: {
		name: string;
		strategy: { duration: string; retry_count: string; type: string };
		signature: { header: string; hash: string };
		disable_endpoint: boolean;
		rate_limit: number;
		rate_limit_duration: string;
	}): Promise<HTTP_RESPONSE> {
		try {
			const response = await this.http.request({
				url: `/groups`,
				body: requestDetails,
				method: 'post'
			});
			return response;
		} catch (error: any) {
			return error;
		}
	}

	async updateProject(requestDetails: {
		name: string;
		strategy: { duration: string; retry_count: string; type: string };
		signature: { header: string; hash: string };
		disable_endpoint: boolean;
		rate_limit: number;
		rate_limit_duration: string;
	}): Promise<HTTP_RESPONSE> {
		try {
			const response = await this.http.request({
				url: `/groups/${this.privateService.activeProjectId}`,
				body: requestDetails,
				method: 'put'
			});
			return response;
		} catch (error: any) {
			return error;
		}
	}
}
