import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { HttpService } from 'src/app/services/http/http.service';
import { FLIPT_API_RESPONSE } from '../models/flipt.model';
import { GROUP } from '../models/group.model';
import { ORGANIZATION_DATA } from '../models/organisation.model';

@Injectable({
	providedIn: 'root'
})
export class PrivateService {
	activeProjectDetails?: GROUP;
	organisationDetails!: ORGANIZATION_DATA;
	apiFlagResponse!: FLIPT_API_RESPONSE;

	constructor(private http: HttpService) {}

	getOrganisation(): ORGANIZATION_DATA {
		let org = localStorage.getItem('CONVOY_ORG');
		return org ? JSON.parse(org) : null;
	}

	urlFactory(level: 'org' | 'org_project'): string {
		const orgId = this.getOrganisation().uid;
		switch (level) {
			case 'org':
				return `/organisations/${orgId}`;
			case 'org_project':
				return `/organisations/${orgId}/projects/${this.activeProjectDetails?.uid}`;
			default:
				return '';
		}
	}

	getConfiguration(): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/configuration`,
					method: 'get'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getApps(requestDetails?: { pageNo?: number; searchString?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${this.urlFactory('org_project')}/apps?sort=AESC&page=${requestDetails?.pageNo || 1}&perPage=20${requestDetails?.searchString ? `&q=${requestDetails?.searchString}` : ''}`,
					method: 'get'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	deleteApp(appID: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${this.urlFactory('org_project')}/apps/${appID}`,
					method: 'delete'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	deleteSubscription(subscriptionId: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const sourceResponse = await this.http.request({
					url: `${this.urlFactory('org_project')}/subscriptions/${subscriptionId}`,
					method: 'delete'
				});

				return resolve(sourceResponse);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getSubscriptions(requestDetails?: { page?: number }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const subscriptionsResponse = await this.http.request({
					url: `${this.urlFactory('org_project')}/subscriptions?page=${requestDetails?.page || 1}`,
					method: 'get'
				});

				return resolve(subscriptionsResponse);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getSources(requestDetails?: { page?: number }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const sourcesResponse = await this.http.request({
					url: `${this.urlFactory('org_project')}/sources?groupId=${this.activeProjectDetails?.uid}&page=${requestDetails?.page}`,
					method: 'get'
				});

				return resolve(sourcesResponse);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getProjectDetails(): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const projectResponse = await this.http.request({
					url: `${this.urlFactory('org')}/projects/${this.activeProjectDetails?.uid}`,
					method: 'get'
				});

				this.activeProjectDetails = projectResponse.data;
				return resolve(projectResponse);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getOrganizations(): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/organisations`,
					method: 'get'
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	logout(): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: '/auth/logout',
					method: 'post',
					body: null
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	addOrganisation(requestDetails: { name: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/organisations`,
					method: 'post',
					body: requestDetails
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getProjects(): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const groupsResponse = await this.http.request({
					url: `${this.urlFactory('org')}/projects`,
					method: 'get'
				});

				return resolve(groupsResponse);
			} catch (error) {
				return reject(error);
			}
		});
	}

	flipt(): Promise<FLIPT_API_RESPONSE> {
		let organisationId: string;
		if (!this.organisationDetails?.uid) {
			const orgDetails = localStorage.getItem('CONVOY_ORG');
			if (orgDetails) organisationId = JSON.parse(orgDetails).uid;
		} else {
			organisationId = this.organisationDetails?.uid;
		}

		return new Promise(async (resolve, reject) => {
			const flagKeys = ['can_create_cli_api_key'];
			const requests: { flagKey: string; entityId: string; context: { group_id: string; organisation_id: string } }[] = [];
			flagKeys.forEach((key: string) =>
				requests.push({
					flagKey: key,
					entityId: key,
					context: {
						group_id: this.activeProjectDetails?.uid || '',
						organisation_id: organisationId
					}
				})
			);

			try {
				const response: any = await this.http.request({ url: `/flags`, method: 'post', body: { requests }, hideNotification: true });
				this.apiFlagResponse = response;
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	async getFlag(flagKey: string): Promise<boolean> {
		try {
			if (!this.apiFlagResponse) await this.flipt();
			const flags = this.apiFlagResponse?.responses;
			return !!flags.find(flag => flag.flagKey === flagKey)?.match;
		} catch (error) {
			return true;
		}
	}

    getEndpoints(token?: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: token ? '/endpoints' : this.urlFactory('org_project') + `/endpoints`,
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
