import {Injectable} from '@angular/core';
import {apiOrigin} from 'src/app/services/api-origin';
import axios from 'axios';
import {ActivatedRoute, Router} from '@angular/router';
import {GeneralService} from '../general/general.service';
import {ProjectService} from 'src/app/private/pages/project/project.service';
import {HTTP_RESPONSE} from 'src/app/models/global.model';

@Injectable({
	providedIn: 'root'
})
export class HttpService {
	APIURL = `${apiOrigin()}/ui`;
	APP_PORTAL_APIURL = `${apiOrigin()}/portal-api`;
	token: string | undefined;
	ownerId: string | undefined;

	constructor(private router: Router, private generalService: GeneralService, private route: ActivatedRoute, private projectService: ProjectService) {
		this.route.queryParams.subscribe(it => {
			if (it.token) this.token = it.token;
			if (it.owner_id) this.ownerId = it.owner_id;
		});
	}

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

		// convert object to query params; JSON-array strings (e.g. status='["Failure","Retry"]')
		// expand into repeated keys. Parts are joined at the end so repeated values are
		// always '&'-separated regardless of key position.
		const parts: string[] = [];

		Object.keys(cleanedQuery).forEach(q => {
			const queryValue = safeQuery[q];
			if (queryValue === undefined) return;
			try {
				const queryItem = JSON.parse(queryValue);
				if (Array.isArray(queryItem)) {
					queryItem.forEach((item: any) => parts.push(`${q}=${item}`));
					return;
				}
			} catch (error) {}
			parts.push(`${q}=${queryValue}`);
		});

		// query items that are real arrays (filtered out of cleanedQuery above)
		Object.keys(safeQuery).forEach((key: any) => {
			const queryValue = safeQuery[key];
			if (queryValue !== undefined && queryValue !== null && typeof queryValue === 'object') {
				queryValue?.forEach?.((item: any) => parts.push(`${key}=${item}`));
			}
		});

		return parts.join('&');
	}

	getOrganisation() {
		let org = localStorage.getItem('CONVOY_ORG');
		return org ? JSON.parse(org) : null;
	}

	// Fail-closed org id accessor for billing calls: any missing/invalid org data
	// throws so callers never build URLs with an empty org id.
	getOrganisationIdOrThrow(): string {
		let org: any = null;
		try {
			org = this.getOrganisation();
		} catch {
			org = null;
		}
		if (!org || !org.uid) {
			throw new Error('Invalid organisation data. Please refresh the page and try again.');
		}
		return org.uid;
	}

	getAccessToken(): string {
		return this.authDetails().access_token || '';
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

		const baseElement = document.querySelector('base');
		const baseHref = baseElement?.getAttribute('href') || '/';
		const rootPath = baseHref.replace(/\/$/, ''); // Remove trailing slash

		const insertRootPath = (url: string) => {
			if (rootPath === '/') return url;
			const urlObj = new URL(url);
			urlObj.pathname = rootPath + urlObj.pathname;
			return urlObj.toString();
		};

		// When token or ownerId is present, use the Portal API URL regardless of other parameters
		if (requestDetails.isPortal) {
			return `${insertRootPath(this.APP_PORTAL_APIURL)}${requestDetails.url}${queryString}`;
		}

		// Handle regular UI paths
		if (!requestDetails.level) {
			return `${insertRootPath(this.APIURL)}${requestDetails.url}${queryString}`;
		}

		const requestPath = this.buildRequestPath(requestDetails.level);
		if (requestPath === 'error') return 'error';

		return `${insertRootPath(this.APIURL)}${requestPath}${requestDetails.url}${queryString}`;
	}

	async request(requestDetails: {
		url: string;
		body?: any;
		method: 'get' | 'post' | 'delete' | 'put';
		isPortal?: boolean;
		hideNotification?: boolean;
		query?: { [param: string]: any };
		level?: 'org' | 'org_project';
		isOut?: boolean;
		returnFullError?: boolean;
	}): Promise<HTTP_RESPONSE> {
		requestDetails.hideNotification = !!requestDetails.hideNotification;

		return new Promise(async (resolve, reject) => {
			try {
				const http = this.setupAxios({ hideNotification: requestDetails.hideNotification });

				// Use token for authorization if available, otherwise use ownerId or access_token
				let authToken = this.getPortalLinkAuthToken() || this.token || this.ownerId;

				if (authToken !== undefined && authToken !== null) {
					requestDetails.isPortal = true;
				}

				// not a portal link innit?
				if (!(this.token || this.ownerId)) {
					authToken = this.authDetails()?.access_token;
					requestDetails.isPortal = false;
				}

				const requestHeader = {
					Authorization: `Bearer ${authToken}`,
					'X-Convoy-Version': '2024-04-01'
				};

				if (requestDetails.url === '/projects/undefined') {
					return;
				}

				// process URL
				const url = this.buildURL(requestDetails);
				if (url === 'error') return;

                // make request
				const { data } = await http.request({
                    method: requestDetails.method,
                    headers: requestHeader,
                    url,
                    data: requestDetails.body
                });
                resolve(data);
            } catch (error) {
                if (axios.isAxiosError(error)) {
                    const msg = error.response?.data?.message;
                    if ('project not found' === msg) {
                        localStorage.removeItem('CONVOY_PROJECT');
                    }
                    if (requestDetails.returnFullError) {
                        return reject(error);
                    } else {
                        // Return the API error message if available, otherwise fall back to error.message
                        const errorMessage = msg || error.message || 'An unexpected error occurred';
                        return reject(errorMessage);
                    }
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
