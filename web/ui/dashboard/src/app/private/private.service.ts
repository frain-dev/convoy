import {EventEmitter, Injectable} from '@angular/core';
import {HTTP_RESPONSE} from 'src/app/models/global.model';
import {HttpService} from 'src/app/services/http/http.service';
import {FLIPT_API_RESPONSE} from '../models/flipt.model';
import {CURSOR} from '../models/global.model';
import {ORGANIZATION_DATA} from '../models/organisation.model';
import {USER} from '../models/user.model';

@Injectable({
	providedIn: 'root'
})
export class PrivateService {
	projects$: EventEmitter<HTTP_RESPONSE> = new EventEmitter();
	showOrgModal: EventEmitter<boolean> = new EventEmitter();
	organisationDetails?: ORGANIZATION_DATA;
	apiFlagResponse!: FLIPT_API_RESPONSE;
	projects!: HTTP_RESPONSE;
	organisations!: HTTP_RESPONSE;
	membership!: HTTP_RESPONSE;
	configutation!: HTTP_RESPONSE;
	showCreateOrgModal = false;
	projectDetails!: HTTP_RESPONSE;
	profileDetails!: HTTP_RESPONSE;
	projectStats!: HTTP_RESPONSE;

	constructor(private http: HttpService) {}

	get getOrganisation(): ORGANIZATION_DATA | null {
		let org = localStorage.getItem('CONVOY_ORG');
		return org ? JSON.parse(org) : null;
	}

	get getUserProfile(): USER | null {
		const authDetails = localStorage.getItem('CONVOY_AUTH');
		return authDetails ? JSON.parse(authDetails) : null;
	}

	// Per-user organization storage
	getUserOrg(userId: string): ORGANIZATION_DATA | null {
		const key = `CONVOY_ORG_${userId}`;
		const org = localStorage.getItem(key);
		return org ? JSON.parse(org) : null;
	}

	setUserOrg(userId: string, org: ORGANIZATION_DATA): void {
		const key = `CONVOY_ORG_${userId}`;
		localStorage.setItem(key, JSON.stringify(org));
		// Also set current session org for compatibility
		localStorage.setItem('CONVOY_ORG', JSON.stringify(org));
	}

	// Per-user project storage
	getUserProject(userId: string): any | null {
		const key = `CONVOY_PROJECT_${userId}`;
		const project = localStorage.getItem(key);
		return project ? JSON.parse(project) : null;
	}

	setUserProject(userId: string, project: any): void {
		const key = `CONVOY_PROJECT_${userId}`;
		localStorage.setItem(key, JSON.stringify(project));
		// Also set current session project for compatibility
		localStorage.setItem('CONVOY_PROJECT', JSON.stringify(project));
	}

	// Clear per-user data when switching users
	clearUserData(userId: string): void {
		localStorage.removeItem(`CONVOY_ORG_${userId}`);
		localStorage.removeItem(`CONVOY_PROJECT_${userId}`);
	}

	setShowOrgModal(value: boolean) {
		this.showOrgModal.emit(value);
	}

	urlFactory(level: 'org' | 'org_project'): string {
		const orgId = this.getOrganisation?.uid;

		switch (level) {
			case 'org':
				return `/organisations/${orgId}`;
			case 'org_project':
				return `/organisations/${orgId}/projects/${this.getProjectDetails?.uid}`;
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

	// Synchronous read of the active project from per-user storage (preferred)
	// or the session cache. Returns null when nothing is cached; callers that
	// need to fetch must call getProject({ projectId }) with an explicit id.
	// The projectResolver populates this cache for /projects/:id routes, and
	// setup-project fetches by its own route id.
	get getProjectDetails() {
		const userId = this.getUserProfile?.uid;
		if (userId) {
			const userProject = this.getUserProject(userId);
			if (userProject) return userProject;
		}

		const localProject = localStorage.getItem('CONVOY_PROJECT');
		if (localProject) return JSON.parse(localProject);

		return null;
	}

	getProject(requestDetails?: { refresh?: boolean; projectId?: string, hideNotification?: boolean }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			if (this.projectDetails && !requestDetails?.refresh) return resolve(this.projectDetails);

			try {
				const projectResponse = await this.http.request({
					url: `/projects/${requestDetails?.projectId}`,
					method: 'get',
					level: 'org',
                    hideNotification: requestDetails?.hideNotification || false,
				});

				this.projectDetails = projectResponse;
				
				// Save to per-user storage
				const user = this.getUserProfile;
				const userId = user?.uid;
				if (userId) {
					this.setUserProject(userId, projectResponse.data);
				} else {
					localStorage.setItem('CONVOY_PROJECT', JSON.stringify(projectResponse.data));
				}
				
				return resolve(projectResponse);
			} catch (error) {
				return reject(error);
			}
		});
	}

	async organisationConfig(organisations: ORGANIZATION_DATA[]) {
		if (!organisations || (organisations && organisations?.length == 0)) return;

		const user = this.getUserProfile;
		const userId = user?.uid;

		// First, try to restore user's last selected org from per-user storage.
		// The list is paginated, so the saved org may not be on this page; trust
		// it by uid and only swap in the page's copy when present (fresher data).
		let rejectedOrgUid: string | undefined;
		if (userId) {
			const userLastOrg = this.getUserOrg(userId);
			if (userLastOrg?.uid) {
				const existingOrg = organisations.find((org: { uid: string }) => org.uid === userLastOrg.uid) || (await this.confirmOrgMembership(userLastOrg));
				if (existingOrg) {
					this.organisationDetails = existingOrg;
					this.setUserOrg(userId, existingOrg);
					return;
				}
				// Saved org was deleted or membership revoked; fall through.
				rejectedOrgUid = userLastOrg.uid;
			}
		}

		// Fallback to current session org if it exists
		const sessionOrg = this.getOrganisation;
		if (sessionOrg?.uid && sessionOrg.uid !== rejectedOrgUid) {
			const existingOrg = organisations.find((org: { uid: string }) => org.uid === sessionOrg.uid) || (await this.confirmOrgMembership(sessionOrg));
			if (existingOrg) {
				if (userId) {
					this.setUserOrg(userId, existingOrg);
				} else {
					localStorage.setItem('CONVOY_ORG', JSON.stringify(existingOrg));
				}
				return;
			}
			// Saved org was deleted or membership revoked; fall through.
		}

		// Default to first org when there is no valid stored selection
		this.organisationDetails = organisations[0];
		if (userId) {
			this.setUserOrg(userId, organisations[0]);
		} else {
			localStorage.setItem('CONVOY_ORG', JSON.stringify(organisations[0]));
		}
		return;
	}

	// Confirms a saved org that is not on the loaded page is still one of the
	// user's organisations by searching their own org list (the backend search
	// matches org id exactly). Returns the fresh copy when confirmed, null when
	// the server says it is gone, and the saved copy on transport errors so a
	// flaky request cannot switch the active org.
	private async confirmOrgMembership(savedOrg: ORGANIZATION_DATA): Promise<ORGANIZATION_DATA | null> {
		try {
			const response = await this.http.request({
				url: `/organisations`,
				method: 'get',
				query: { perPage: 20, q: savedOrg.uid }
			});
			return (response.data?.content || []).find((org: ORGANIZATION_DATA) => org.uid === savedOrg.uid) || null;
		} catch {
			return savedOrg;
		}
	}

	getOrganizations(requestDetails?: { refresh?: boolean; q?: string; next_page_cursor?: string; perPage?: number }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			const isFirstPage = !requestDetails?.q && !requestDetails?.next_page_cursor;
			if (this.organisations && !requestDetails?.refresh && isFirstPage) return resolve(this.organisations);

			try {
				const query: { perPage: number; q?: string; next_page_cursor?: string } = { perPage: requestDetails?.perPage || 20 };
				if (requestDetails?.q) query.q = requestDetails.q;
				if (requestDetails?.next_page_cursor) query.next_page_cursor = requestDetails.next_page_cursor;

				const response = await this.http.request({
					url: `/organisations`,
					method: 'get',
					query
				});

				// Only the unfiltered first page seeds the cached org selection state.
				if (isFirstPage) {
					await this.organisationConfig(response.data?.content);
					this.organisations = response;
				}
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

	getProjects(requestDetails?: { refresh: boolean }): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			if (this.projects && !requestDetails?.refresh) return resolve(this.projects);

			try {
				const projectsResponse = await this.http.request({
					url: `/projects`,
					method: 'get',
					level: 'org'
				});

				this.projects = projectsResponse;
				this.projects$.emit(projectsResponse);
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
						group_id: this.getProjectDetails?.uid || '',
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
			const userId = requestDetails?.userId;
			if (!userId || userId === 'undefined') {
				return reject(new Error('missing user id'));
			}

			if (this.profileDetails && !requestDetails.refresh) return resolve(this.profileDetails);

			try {
				const response = await this.http.request({
					url: `/users/${userId}/profile`,
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

	getSources(requestDetails?: CURSOR & { q?: string }): Promise<HTTP_RESPONSE> {
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

	clearCache(clearLocalStorage: boolean = false, userId?: string): void {
		// Clear all in-memory cached data
		this.projects = undefined as any;
		this.organisations = undefined as any;
		this.projectDetails = undefined as any;
		this.membership = undefined as any;
		this.organisationDetails = undefined;
		this.profileDetails = undefined as any;
		this.projectStats = undefined as any;
		this.configutation = undefined as any;
		this.apiFlagResponse = undefined as any;
		
		// Only clear localStorage items when explicitly requested (e.g., when switching users)
		if (clearLocalStorage) {
			// Clear current session data
			localStorage.removeItem('CONVOY_ORG');
			localStorage.removeItem('CONVOY_PROJECT');
			// If userId provided, also clear that user's per-user data (when switching away from them)
			if (userId) {
				this.clearUserData(userId);
			}
		}
	}

	deleteProject(): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const sourceResponse = await this.http.request({
					url: ``,
					method: 'delete',
					level: 'org_project'
				});

				return resolve(sourceResponse);
			} catch (error) {
				return reject(error);
			}
		});
	}

	deleteEndpoint(endpointId: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/endpoints/${endpointId}`,
					method: 'delete',
					level: 'org_project'
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}

    async getEventTypes(): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `/event-types`,
					method: 'get',
                    level: 'org_project',
                    hideNotification: true,
				});

				return resolve(response);
			} catch (error) {
				return reject(error);
			}
		});
	}
}
