import { Injectable } from '@angular/core';
import { environment } from 'src/environments/environment';
import axios from 'axios';
import { ActivatedRoute, Router } from '@angular/router';
import { GeneralService } from '../general/general.service';
import { ProjectService } from 'src/app/private/pages/project/project.service';
import { HTTP_RESPONSE } from 'src/app/models/global.model';

@Injectable({
	providedIn: 'root'
})
export class HttpService {
	APIURL = `${environment.production ? location.origin : 'http://localhost:5005'}/ui`;
	APP_PORTAL_APIURL = `${environment.production ? location.origin : 'http://localhost:5005'}/portal-api`;
	token = this.route.snapshot.queryParams?.token;
	ownerId = this.route.snapshot.queryParams?.owner_id;

	constructor(private router: Router, private generalService: GeneralService, private route: ActivatedRoute, private projectService: ProjectService) {}

	authDetails(): { access_token?: string; refresh_token?: string; authState: boolean } {
		const authDetails = localStorage.getItem('CONVOY_AUTH_TOKENS');
		if (authDetails && authDetails !== 'undefined') {
			const token = JSON.parse(authDetails);
			return { access_token: token.access_token, refresh_token: token.refresh_token, authState: true };
		} else {
			return { authState: false };
		}
	}

	buildRequestQuery(query?: { [k: string]: any }) {
		// Create a new object if query is undefined
		const safeQuery: { [k: string]: any } = query || {};

		// add portal link query if available
		if (this.token) safeQuery.token = this.token;

		// add owner_id if available
		if (this.ownerId) safeQuery.owner_id = this.ownerId;

		// remove empty data and objects in object
		const cleanedQuery = Object.fromEntries(Object.entries(safeQuery).filter(([_, q]) => q !== '' && q !== undefined && q !== null && typeof q !== 'object'));

		// convert object to query param
		let cleanedQueryString: string = '';

		Object.keys(cleanedQuery).forEach((q, i) => {
			try {
				const queryValue = safeQuery[q];
				if (queryValue !== undefined) {
					const queryItem = JSON.parse(queryValue);
					queryItem.forEach((item: any) => (cleanedQueryString += `${q}=${item}${Object.keys(cleanedQuery).length - 1 !== i ? '&' : ''}`));
				}
			} catch (error) {
				const queryValue = safeQuery[q];
				if (queryValue !== undefined) {
					cleanedQueryString += `${q}=${queryValue}${Object.keys(cleanedQuery).length - 1 !== i ? '&' : ''}`;
				}
			}
		});

		// for query items with arrays, process them into a string
		let queryString = '';
		Object.keys(safeQuery).forEach((key: any) => {
			const queryValue = safeQuery[key];
			if (queryValue !== undefined && typeof queryValue === 'object') {
				queryValue?.forEach((item: any) => (queryString += `&${key}=${item}`));
			}
		});

		return cleanedQueryString + queryString;
	}

	getOrganisation() {
		let org = localStorage.getItem('CONVOY_ORG');
		return org ? JSON.parse(org) : null;
	}

	getPortalLinkAuthToken() {
		return localStorage.getItem('CONVOY_PORTAL_LINK_AUTH_TOKEN');
	}

	getProject() {
		let project = localStorage.getItem('CONVOY_PROJECT');
		return project ? JSON.parse(project) : null;
	}

	buildRequestPath(level?: 'org' | 'org_project'): string {
		if (!level) return '';
		const orgId = this.getOrganisation()?.uid;
		const projectId = this.getProject()?.uid;

		if (level === 'org' && !orgId) return 'error';
		if (level === 'org_project' && (!orgId || !projectId)) return 'error';

		switch (level) {
			case 'org':
				return `/organisations/${orgId}`;
			case 'org_project':
				return `/organisations/${orgId}/projects/${projectId}`;
			default:
				return '';
		}
	}

	buildURL(requestDetails: any): string {
		if (requestDetails.isOut) return requestDetails.url;

		// Make sure we have a query object
		const query = requestDetails.query || {};

		// Add token and owner_id to query if they exist
		if (this.token) {
			query['token'] = this.token;
		}
		if (this.ownerId) {
			query['owner_id'] = this.ownerId;
		}

		// Format query string if we have any query params
		const queryString = Object.keys(query).length > 0 ? '?' + this.buildRequestQuery(query) : '';

		// When token or ownerId is present, use the Portal API URL regardless of other parameters
		if (this.token || this.ownerId) {
			return `${this.APP_PORTAL_APIURL}${requestDetails.url}${queryString}`;
		}

		// Handle regular UI paths
		if (!requestDetails.level) {
			return `${this.APIURL}${requestDetails.url}${queryString}`;
		}

		const requestPath = this.buildRequestPath(requestDetails.level);
		if (requestPath === 'error') return 'error';

		return `${this.APIURL}${requestPath}${requestDetails.url}${queryString}`;
	}

	async request(requestDetails: { url: string; body?: any; method: 'get' | 'post' | 'delete' | 'put'; hideNotification?: boolean; query?: { [param: string]: any }; level?: 'org' | 'org_project'; isOut?: boolean }): Promise<HTTP_RESPONSE> {
		requestDetails.hideNotification = !!requestDetails.hideNotification;

		return new Promise(async (resolve, reject) => {
			try {
				const http = this.setupAxios({ hideNotification: requestDetails.hideNotification });

				// Use token for authorization if available, otherwise use ownerId or access_token
				let authToken = this.token || this.ownerId || this.authDetails()?.access_token;
				if (this.ownerId || this.token) {
					authToken = this.getPortalLinkAuthToken();
				}

				const requestHeader = {
					Authorization: `Bearer ${authToken}`,
					'X-Convoy-Version': '2024-04-01'
				};

				// process URL
				const url = this.buildURL(requestDetails);
				if (url === 'error') return;

				// make request
				const { data } = await http.request({ method: requestDetails.method, headers: requestHeader, url, data: requestDetails.body });
				resolve(data);
			} catch (error) {
				if (axios.isAxiosError(error)) {
					return reject(error.message);
				} else {
					console.log('unexpected error: ', error);
					return reject('An unexpected error occurred');
				}
			}
		});
	}

	setupAxios(requestDetails: { hideNotification: any }) {
		const http = axios.create();

		// Interceptor
		http.interceptors.response.use(
			request => {
				return request;
			},
			error => {
				if (axios.isAxiosError(error)) {
					const errorResponse: any = error.response;
					let errorMessage: any = errorResponse?.data ? errorResponse.data.message : error.message;

					if (error.response?.status == 401 && !this.router.url.split('/')[1].includes('portal')) {
						this.logUserOut();
						return Promise.reject(error);
					}

					if (!requestDetails.hideNotification) {
						this.generalService.showNotification({
							message: errorMessage,
							style: 'error'
						});
					}
					return Promise.reject(error);
				}

				if (!requestDetails.hideNotification) {
					let errorMessage: string;
					error.error?.message ? (errorMessage = error.error?.message) : (errorMessage = 'An error occured, please try again');
					this.generalService.showNotification({
						message: errorMessage,
						style: 'error'
					});
				}
				return Promise.reject(error);
			}
		);

		return http;
	}

	logUserOut() {
		// save previous location before session timeout
		if (this.router.url.split('/')[1] !== 'login') localStorage.setItem('CONVOY_LAST_AUTH_LOCATION', location.href);

		// then logout
		this.router.navigate(['/login'], { replaceUrl: true });
	}
}
