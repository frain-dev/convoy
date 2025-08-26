import {Injectable} from '@angular/core';
import {HTTP_RESPONSE} from 'src/app/models/global.model';
import {HttpService} from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class GoogleOAuthSetupService {
	constructor(private http: HttpService) {}

	completeSetup(idToken: string, businessName: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: '/auth/google/setup',
					body: {
						business_name: businessName,
						id_token: idToken
					},
					method: 'post'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
