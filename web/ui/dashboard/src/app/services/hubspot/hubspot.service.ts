import { Injectable } from '@angular/core';
import axios from 'axios';
import { environment } from 'src/environments/environment';

@Injectable({
	providedIn: 'root'
})
export class HubspotService {
	constructor() {}

	async sendWelcomeEmail(emailData: { email: string; firstname: string; lastname: string }) {
		return new Promise(async (resolve, reject) => {
			try {
				const http = axios.create();
				const { data } = await http.get(`https://faas-fra1-afec6ce7.doserverless.co/api/v1/web/fn-8f44e6aa-e5d6-4e31-b781-5080c050bb37/welcome-user/welcome-mail?email=${emailData.email}&firstname=${emailData.firstname}&lastname=${emailData.lastname}`);
				resolve(data);
			} catch (error) {
				if (axios.isAxiosError(error)) {
					console.log('hubspot error message: ', error.message);
					return reject(error.message);
				} else {
					console.log('hubspot unexpected error: ', error);
					return reject('An hubspot unexpected error occurred');
				}
			}
		});
	}
}
