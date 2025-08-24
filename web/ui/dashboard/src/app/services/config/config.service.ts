import {Injectable} from '@angular/core';
import {HttpService} from '../http/http.service';

export interface GoogleOAuthConfig {
  enabled: boolean;
  client_id: string;
  redirect_url: string;
}

export interface AppConfig {
  auth: {
    google_oauth: GoogleOAuthConfig;
    saml?: { enabled: boolean };
    is_signup_enabled?: boolean;
    [key: string]: any;
  };
  [key: string]: any;
}

@Injectable({
  providedIn: 'root'
})
export class ConfigService {
  private config: AppConfig | null = null;

  constructor(private http: HttpService) {}

  async getConfig(): Promise<AppConfig> {
    if (this.config) {
      return this.config;
    }

    try {

      const response = await this.http.request({
        url: '/configuration/auth',
        method: 'get'
      });

      if (response.data) {
        this.config = {
          auth: {
            google_oauth: {
              enabled: response.data.google_oauth?.enabled || false,
              client_id: response.data.google_oauth?.client_id || '',
              redirect_url: response.data.google_oauth?.redirect_url || ''
            },
            saml: response.data.saml || { enabled: false },
            is_signup_enabled: response.data.is_signup_enabled || false
          }
        } as AppConfig;
      } else {

        this.config = this.getDefaultConfig();
      }
      return this.config;
    } catch (error) {
      console.error('Failed to fetch config:', error);

      this.config = this.getDefaultConfig();
      return this.config;
    }
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

  private getDefaultConfig(): AppConfig {
    return {
      auth: {
        google_oauth: {
          enabled: false,
          client_id: '',
          redirect_url: ''
        }
      }
    };
  }
}
