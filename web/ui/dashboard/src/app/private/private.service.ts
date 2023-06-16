import { Injectable } from '@angular/core';
import { Router } from '@angular/router';
import { HTTP_RESPONSE } from 'src/app/models/global.model';
import { HttpService } from 'src/app/services/http/http.service';
import { FLIPT_API_RESPONSE } from '../models/flipt.model';
import { CURSOR } from '../models/global.model';
import { PROJECT } from '../models/project.model';
import { ORGANIZATION_DATA } from '../models/organisation.model';
import { ProjectService } from './pages/project/project.service';
import { USER } from '../models/user.model';

@Injectable({
	providedIn: 'root'
})
export class PrivateService {
	activeProjectDetails?: PROJECT; // we should depricate this
	organisationDetails!: ORGANIZATION_DATA;
	apiFlagResponse!: FLIPT_API_RESPONSE;
	projects: PROJECT[] = [];
	organisations!: HTTP_RESPONSE;
	membership!: HTTP_RESPONSE;
	configutation!: HTTP_RESPONSE;
	showCreateOrgModal = false;
	projectDetails!: HTTP_RESPONSE;
	profileDetails!: HTTP_RESPONSE;
	projectStats!: HTTP_RESPONSE;

	constructor(private http: HttpService, private router: Router, private projectService: ProjectService) {}

	get getOrganisation(): ORGANIZATION_DATA | null {
		let org = localStorage.getItem('CONVOY_ORG');
		return org ? JSON.parse(org) : null;
	}

	get getUserProfile(): USER | null {
		const authDetails = localStorage.getItem('CONVOY_AUTH');
		return authDetails ? JSON.parse(authDetails) : false;
	}

	urlFactory(level: 'org' | 'org_project'): string {
		const orgId = this.getOrganisation?.uid;

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
			if (this.configutation) return resolve(this.configutation);

			try {
				const response = await this.http.request({
					url: `/configuration`,
					method: 'get'
				});

				this.configutation = response;
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

	getSubscriptions(requestDetails?: any): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				if (!requestDetails) requestDetails = { next_page_cursor: 'FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF', direction: 'next' };

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

	getProjectDetails(requestDetails?: { refresh?: boolean; projectId?: string }): Promise<HTTP_RESPONSE> {
		const projectId = this.router.url.split('/')[2];

		return new Promise(async (resolve, reject) => {
			if (this.projectDetails && !requestDetails?.refresh) return resolve(this.projectDetails);

			try {
				const projectResponse = await this.http.request({
					url: `/projects/${requestDetails?.projectId || this.activeProjectDetails?.uid || projectId}`,
					method: 'get',
					level: 'org'
				});

				this.activeProjectDetails = projectResponse.data; // we should depricate this
				this.projectService.activeProjectDetails = projectResponse.data;

				this.projectDetails = projectResponse;
				return resolve(projectResponse);
			} catch (error) {
				return reject(error);
			}
		});
	}

	async organisationConfig(organisations: ORGANIZATION_DATA[]) {
		if (!organisations || (organisations && organisations?.length == 0)) return;

		const existingOrg = organisations.find((org: { uid: string }) => org.uid === this.getOrganisation?.uid);
		if (existingOrg) return localStorage.setItem('CONVOY_ORG', JSON.stringify(existingOrg));

		this.organisationDetails = organisations[0];
		localStorage.setItem('CONVOY_ORG', JSON.stringify(organisations[0]));
		return;
	}

	getOrganizations(requestDetails?: { refresh: boolean }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			if (this.organisations && !requestDetails?.refresh) return resolve(this.organisations);

			try {
				const response = await this.http.request({
					url: `/organisations`,
					method: 'get'
				});

				await this.organisationConfig(response.data?.content);
				this.organisations = response;
				if (!response.data.content.length) return this.router.navigateByUrl('/get-started');
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getTeamMembers(requestDetails?: { q?: string; page?: number; userID?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/members`,
					method: 'get',
					level: 'org',
					query: requestDetails
				});
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getOrganizationMembership(requestDetails?: { refresh: boolean }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			if (!this.organisations?.data?.content.length) return resolve(this.membership);
			if (this.membership && !requestDetails?.refresh) return resolve(this.membership);

			try {
				const response = await this.getTeamMembers({ userID: this.getUserProfile?.uid });
				this.membership = response;
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

	getUserDetails(requestDetails: { userId: string; refresh?: boolean }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			if (this.profileDetails && !requestDetails.refresh) return resolve(this.profileDetails);

			try {
				const response = await this.http.request({
					url: `/users/${requestDetails.userId}/profile`,
					method: 'get'
				});

				this.profileDetails = response;
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

	getEndpoints(requestDetails?: CURSOR & { q?: string }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				if (!requestDetails?.next_page_cursor && !requestDetails?.prev_page_cursor) requestDetails = { next_page_cursor: 'FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF', direction: 'next', q: requestDetails?.q };

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

	getSources(requestDetails?: CURSOR): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				if (!requestDetails) requestDetails = { next_page_cursor: 'FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF', direction: 'next' };

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

	async getRefreshToken() {
		let authTokens = localStorage.CONVOY_AUTH_TOKENS;
		authTokens = authTokens ? JSON.parse(authTokens) : false;

		return new Promise(async (resolve, reject) => {
			if (!authTokens) return reject();

			try {
				const refreshedTokens = await this.http.request({
					url: `/auth/token/refresh`,
					method: 'post',
					body: authTokens
				});
				localStorage.setItem('CONVOY_AUTH_TOKENS', JSON.stringify(refreshedTokens.data));

				return resolve(refreshedTokens);
			} catch (error) {
				reject(error);
			}
		});
	}

	getProjectStat(requestDetails?: { refresh: boolean }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			if (this.projectStats && !requestDetails?.refresh) return resolve(this.projectStats);

			try {
				const response = await this.http.request({
					url: `/stats`,
					method: 'get',
					level: 'org_project'
				});

				this.projectStats = response;
				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
