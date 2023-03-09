import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { environment } from 'src/environments/environment';
import axios from 'axios';
import { ActivatedRoute, Router } from '@angular/router';
import { GeneralService } from '../general/general.service';
import { JwtHelperService } from '@auth0/angular-jwt';
import { differenceInSeconds } from 'date-fns';
import { ProjectService } from 'src/app/private/pages/project/project.service';

@Injectable({
	providedIn: 'root'
})
export class HttpService {
	APIURL = `${environment.production ? location.origin : 'http://localhost:5005'}/ui`;
	APP_PORTAL_APIURL = `${environment.production ? location.origin : 'http://localhost:5005'}/portal-api`;
	token = this.route.snapshot.queryParams?.token;
	checkTokenTimeout: any;

	public jwtHelper: JwtHelperService = new JwtHelperService();

	constructor(private router: Router, private generalService: GeneralService, private route: ActivatedRoute, private projectService: ProjectService) {}

	authDetails() {
		const authDetails = localStorage.getItem('CONVOY_AUTH_TOKENS');
		if (authDetails && authDetails !== 'undefined') {
			const token = JSON.parse(authDetails);
			return { access_token: token.access_token, refresh_token: token.refresh_token, authState: true };
		} else {
			return { authState: false };
		}
	}

	buildRequestQuery(query?: { [k: string]: any }) {
		if (!query || (query && Object.getOwnPropertyNames(query).length == 0)) return '';

		// add portal link query if available
		if (this.token) query.token = this.token;

		// remove empty data and objects in object
		const cleanedQuery = Object.fromEntries(Object.entries(query).filter(([_, q]) => q !== '' && q !== undefined && q !== null && typeof q !== 'object'));

		// for query items with arrays, process them into a string
		let queryString = '';
		Object.keys(query).forEach((key: any) => (typeof query[key] === 'object' ? query[key]?.forEach((item: any) => (queryString += `&${key}=${item}`)) : false));

		// convert object to query param
		return new URLSearchParams(cleanedQuery).toString() + queryString;
	}

	getOrganisation() {
		let org = localStorage.getItem('CONVOY_ORG');
		return org ? JSON.parse(org) : null;
	}

	buildRequestPath(level?: 'org' | 'org_project'): string {
		const orgId = this.getOrganisation().uid;

		switch (level) {
			case 'org':
				return `/organisations/${orgId}`;
			case 'org_project':
				return `/organisations/${orgId}/projects/${this.projectService.activeProjectDetails?.uid}`;
			default:
				return '';
		}
	}

	buildURL(requestDetails: any): string {
		if (requestDetails.isOut) return requestDetails.url;

		if (this.token) return `${this.token ? this.APP_PORTAL_APIURL : this.APIURL}${requestDetails.url}?${this.buildRequestQuery(requestDetails.query)}}`;

		return `${this.APIURL}${this.buildRequestPath(requestDetails.level)}${requestDetails.url}?${this.buildRequestQuery(requestDetails.query)}}`;
	}

	async request(requestDetails: { url: string; body?: any; method: 'get' | 'post' | 'delete' | 'put'; hideNotification?: boolean; query?: { [param: string]: any }; level?: 'org' | 'org_project'; isOut?: boolean }): Promise<HTTP_RESPONSE> {
		requestDetails.hideNotification = !!requestDetails.hideNotification;

		return new Promise(async (resolve, reject) => {
			try {
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
								this.getRefreshToken();
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

				const requestHeader = {
					Authorization: `Bearer ${this.token || this.authDetails()?.access_token}`
				};

				// make request
				const { data } = await http.request({
					method: requestDetails.method,
					headers: requestHeader,
					url: this.buildURL(requestDetails),
					data: requestDetails.body
				});
				resolve(data);
			} catch (error) {
				if (axios.isAxiosError(error)) {
					console.log('error message: ', error.message);
					return reject(error.message);
				} else {
					console.log('unexpected error: ', error);
					return reject('An unexpected error occurred');
				}
			}
		});
	}

	async getRefreshToken() {
		try {
			const refreshedTokens = await this.request({
				url: `/auth/token/refresh`,
				method: 'post',
				body: this.authDetails()
			});
			localStorage.setItem('CONVOY_AUTH_TOKENS', JSON.stringify(refreshedTokens.data));
			window.location.reload();
		} catch {
			this.initiateLogout();
		}
	}

	checkIfTokenIsExpired() {
		const currentTime = new Date();
		const tokenExpiryTime = this.jwtHelper.getTokenExpirationDate(this.authDetails().access_token);
		if (tokenExpiryTime) {
			const expiryPeriodInSeconds = differenceInSeconds(tokenExpiryTime, currentTime);
			if (expiryPeriodInSeconds <= 180) {
				clearTimeout(this.checkTokenTimeout);
				this.getRefreshToken();
			}
		}
	}

	initiateLogout() {
		// save previous location before session timeout
		if (this.router.url.split('/')[1] !== 'login') localStorage.setItem('CONVOY_LAST_AUTH_LOCATION', location.href);

		// then logout
		this.router.navigate(['/login'], { replaceUrl: true });
		localStorage.removeItem('CONVOY_AUTH_TOKENS');
		this.generalService.showNotification({
			message: 'Authorization Failed',
			style: 'error'
		});
	}
}
