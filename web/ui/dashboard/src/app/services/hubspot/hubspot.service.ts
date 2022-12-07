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
				const requestHeader = {
					Authorization: `Bearer ${environment.hubspot}`,
					'Content-Type': 'application/json'
				};

				const { data, status } = await http.request({
					method: 'post',
					headers: requestHeader,
					url: 'https://api.hubapi.com/contacts/v1/contact',
					data: {
						properties: [
							{
								property: 'email',
								value: emailData.email
							},
							{
								property: 'firstname',
								value: emailData.firstname
							},
							{
								property: 'lastname',
								value: emailData.lastname
							}
						]
					}
				});
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
