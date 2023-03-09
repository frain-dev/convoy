import { Injectable } from '@angular/core';
import { Router } from '@angular/router';
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
	projects: GROUP[] = [];

	constructor(private http: HttpService, private router: Router) {}

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

	deleteSubscription(subscriptionId: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const sourceResponse = await this.http.request({
					url: `/subscriptions/${subscriptionId}`,
					method: 'delete',
					level: 'org_project'
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
					url: `/subscriptions`,
					method: 'get',
					level: 'org_project',
					query: requestDetails
				});

				return resolve(subscriptionsResponse);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getProjectDetails(): Promise<HTTP_RESPONSE> {
		const projectId = this.router.url.split('/')[2];

		return new Promise(async (resolve, reject) => {
			try {
				const projectResponse = await this.http.request({
					url: `/projects/${this.activeProjectDetails?.uid || projectId}`,
					method: 'get',
					level: 'org'
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
				const projectsResponse = await this.http.request({
					url: `/projects`,
					method: 'get',
					level: 'org'
				});

				this.projects = projectsResponse.data;
				return resolve(projectsResponse);
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
				const response: any = await this.http.request({ url: `/flags`, method: 'post', body: { requests }, hideNotification: true, isOut: true });
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
			return false;
		}
	}

	getUserDetails(requestDetails: { userId: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/users/${requestDetails.userId}/profile`,
					method: 'get'
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getEndpoints(requestDetails?: { page?: number; q?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/endpoints`,
					method: 'get',
					query: requestDetails,
					level: 'org_project'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getSources(requestDetails?: { page?: number }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const sourcesResponse = await this.http.request({
					url: `/sources`,
					method: 'get',
					level: 'org_project',
					query: requestDetails
				});

				return resolve(sourcesResponse);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
