import { HttpClient, HttpHeaders, HttpResponse } from '@angular/common/http';
import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { environment } from 'src/environments/environment';

@Injectable({
	providedIn: 'root'
})
export class HttpService {
	APIURL = `${environment.production ? location.origin : 'http://localhost:5005'}/ui`;

	constructor(private httpClient: HttpClient) {}

	authDetails() {
		const authDetails = localStorage.getItem('CONVOY_AUTH');
		if (authDetails) {
			const { token } = JSON.parse(authDetails);
			return { token, authState: true };
		} else {
			return { authState: false };
		}
	}

	request(requestDetails: { url: string; body?: any; method: 'get' | 'post' | 'delete' | 'put' }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const requestHeader = new HttpHeaders({
					Authorization: `Bearer ${this.authDetails().token}`
				});
				const requestResponse: any = await this.httpClient.request(requestDetails.method, this.APIURL + requestDetails.url, { headers: requestHeader, body: requestDetails.body }).toPromise();
				return resolve(requestResponse);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
