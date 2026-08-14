import {Injectable} from '@angular/core';
import {HttpService} from '../http/http.service';
import {BillingStrategy} from 'src/app/models/billing.model';

export interface GoogleOAuthConfig {
	enabled: boolean;
	client_id: string;
	redirect_url: string;
}

export interface AppConfig {
	auth: {
		google_oauth: GoogleOAuthConfig;
		sso?: { enabled: boolean; redirect_url?: string };
		is_signup_enabled?: boolean;
		[key: string]: any;
	};
	billing_strategy?: BillingStrategy;
	[key: string]: any;
}

@Injectable({
	providedIn: 'root'
})
export class ConfigService {
	private config: AppConfig | null = null;
	private inFlight: Promise<AppConfig> | null = null;

	constructor(private http: HttpService) {}

	/**
	 * Fetches auth configuration. When slug is provided (cloud billing), requests config for that workspace
	 * and returns sso.enabled for that org; does not use cache so each slug lookup is fresh.
	 */
	async getConfig(slug?: string): Promise<AppConfig> {
		if (!slug && this.config) {
			return this.config;
		}

		// Share one in-flight request for the default (no-slug) config so
		// APP_INITIALIZER preload and login/signup wait on the same response.
		if (!slug && this.inFlight) {
			return this.inFlight;
		}

		const request = this.fetchConfig(slug);
		if (!slug) {
			this.inFlight = request.finally(() => {
				this.inFlight = null;
			});
			return this.inFlight;
		}

		return request;
	}

	async getGoogleOAuthConfig(): Promise<GoogleOAuthConfig> {
		const config = await this.getConfig();
		return config.auth.google_oauth;
	}

	isGoogleOAuthEnabled(): boolean {
		if (!this.config) return false;
		return this.config.auth?.google_oauth?.enabled || false;
	}

	getGoogleClientId(): string {
		if (!this.config) return '';
		return this.config.auth?.google_oauth?.client_id || '';
	}

	private async fetchConfig(slug?: string): Promise<AppConfig> {
		try {
			const query = slug ? { slug: slug.trim() } : {};
			const response = await this.http.request({
				url: '/configuration/auth',
				method: 'get',
				query,
				hideNotification: true
			});

			if (response.data) {
				const config = {
					billing_strategy: response.data.billing_strategy,
					auth: {
						google_oauth: {
							enabled: response.data.google_oauth?.enabled || false,
							client_id: response.data.google_oauth?.client_id || '',
							redirect_url: response.data.google_oauth?.redirect_url || ''
						},
						sso: {
							enabled: response.data.sso?.enabled ?? false,
							redirect_url: response.data.sso?.redirect_url || ''
						},
						is_signup_enabled: response.data.is_signup_enabled || false
					}
				} as AppConfig;
				if (!slug) {
					this.config = config;
				}
				return config;
			}

			throw new Error('No config data in response');
		} catch (error) {
			console.error('Failed to fetch config:', error);
			return this.getDefaultConfig();
		}
	}

	private getDefaultConfig(): AppConfig {
		return {
			auth: {
				google_oauth: {
					enabled: false,
					client_id: '',
					redirect_url: ''
				},
				sso: { enabled: false, redirect_url: '' }
			}
		};
	}
}
