import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class CreateProjectService {
	constructor(private http: HttpService) {}

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
}
