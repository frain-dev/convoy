import {Injectable} from '@angular/core';
import {ConfigService} from '../config/config.service';

declare global {
  interface Window {
    google: any;
  }
}

export interface GoogleCredential {
  credential: string | null;
  select_by: string;
}

@Injectable({
  providedIn: 'root'
})
export class GoogleOAuthService {
  private clientId: string = '';
  private isInitialized: boolean = false;

  constructor(private configService: ConfigService) {}

  async initialize(): Promise<void> {
    try {
      const config = await this.configService.getGoogleOAuthConfig();
      if (config.enabled && config.client_id) {
        this.clientId = config.client_id;
        await this.loadGoogleIdentityServices();
        this.isInitialized = true;
      }
    } catch (error) {
      console.error('Failed to initialize Google OAuth:', error);
    }
  }

  private loadGoogleIdentityServices(): Promise<void> {
    return new Promise((resolve, reject) => {
      // Check if already loaded
      if (window.google && window.google.accounts) {
        resolve();
        return;
      }

      // Load Google Identity Services script
      const script = document.createElement('script');
      script.src = 'https://accounts.google.com/gsi/client';
      script.async = true;
      script.defer = true;

      script.onload = () => {
        this.initializeGoogleIdentity();
        resolve();
      };

      script.onerror = () => {
        reject(new Error('Failed to load Google Identity Services'));
      };

      document.head.appendChild(script);
    });
  }

  private initializeGoogleIdentity(): void {
    if (window.google && window.google.accounts) {
      window.google.accounts.id.initialize({
        client_id: this.clientId,
        callback: this.handleCredentialResponse.bind(this),
        auto_select: false,
        cancel_on_tap_outside: true,
      });
    }
  }

  private handleCredentialResponse(response: GoogleCredential): void {
    // Global callback handler - not used in our implementation
  }

  async signIn(): Promise<GoogleCredential> {
    if (!this.isInitialized) {
      throw new Error('Google OAuth not initialized');
    }

    return new Promise((resolve, reject) => {
      if (window.google && window.google.accounts) {
        // Store the original callback
        const originalCallback = window.google.accounts.id.callback;

        // Set up a one-time callback for this sign-in
        window.google.accounts.id.initialize({
          client_id: this.clientId,
          callback: (response: GoogleCredential) => {
            // Restore the original callback
            if (originalCallback) {
              window.google.accounts.id.initialize({
                client_id: this.clientId,
                callback: originalCallback,
                auto_select: false,
                cancel_on_tap_outside: true,
              });
            }
            resolve(response);
          },
          auto_select: false,
          cancel_on_tap_outside: true,
        });

        // Prompt for sign-in
        window.google.accounts.id.prompt((notification: any) => {

          if (notification.g === 'not_displayed') {
            reject(new Error('Google Sign-In prompt not displayed'));
          } else if (notification.g === 'skipped') {
            if (notification.l === 'tap_outside') {
              reject(new Error('Google Sign-In was cancelled by user'));
            } else {
              reject(new Error('FedCM authentication blocked by browser'));
            }
          } else if (notification.g === 'dismissed') {
            if (notification.i === 'credential_returned') {

            } else {
              reject(new Error('Google Sign-In was cancelled by user'));
            }
          }
        });
      } else {
        reject(new Error('Google Identity Services not available'));
      }
    });
  }

  isReady(): boolean {
    return this.isInitialized;
  }
}
