import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
  providedIn: 'root'
})
export class CreateOrganisationService {

  constructor(private http:HttpService) { }
  
  async addOrganisation(requestDetails: { name: string }): Promise<HTTP_RESPONSE> {
		try {
			const response = await this.http.request({
				url: `/organisations`,
				method: 'post',
				body: requestDetails
			});
			return response;
		} catch (error: any) {
			return error;
		}
	}
}
