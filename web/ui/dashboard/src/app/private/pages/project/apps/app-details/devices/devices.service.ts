import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { PrivateService } from 'src/app/private/private.service';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class DevicesService {
	constructor(private http: HttpService, private privateService: PrivateService) {}

	getAppDevices(appId: string, token?: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: token ? '/apps/devices' : `${this.privateService.urlFactory('org_project')}/apps/${appId}/devices`,
					method: 'get',
					token
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	async getAppPortalApp(token: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/apps`,
					method: 'get',
					token
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
