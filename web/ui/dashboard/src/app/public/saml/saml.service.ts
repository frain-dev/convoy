import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/global.model';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class SamlService {
	constructor(private http: HttpService) {}

	authenticateWithSaml(accessCode: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/auth/saml?saml_access_code=${accessCode}`,
					method: 'get'
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
