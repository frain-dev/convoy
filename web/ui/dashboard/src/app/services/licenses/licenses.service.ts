import { Injectable } from '@angular/core';
import { HttpService } from '../http/http.service';
import { HTTP_RESPONSE } from 'src/app/models/global.model';

@Injectable({
	providedIn: 'root'
})
export class LicensesService {
	constructor(private http: HttpService) {}

	getLicenses(): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/license/features`,
					method: 'get'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}


	async setLicenses() {
		try {
			const response = await this.getLicenses();

			let allowedLicenses: any[] = [];
			Object.entries(response.data).forEach(([key, entry]: any) => {
				if (entry.allowed) allowedLicenses.push(key);
			});

			localStorage.setItem('licenses', JSON.stringify(allowedLicenses));
		} catch {}
	}

	hasLicense(license: string) {
		const savedLicenses = localStorage.getItem('licenses');
		if (savedLicenses) {
			const licenses = JSON.parse(savedLicenses);
			const userHasLicense = licenses.includes(license);

			return userHasLicense;
		}

		return false;
	}
}
