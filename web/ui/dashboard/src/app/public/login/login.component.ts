import {CommonModule} from '@angular/common';
import {AfterViewInit, Component, OnInit} from '@angular/core';
import {FormBuilder, FormGroup, ReactiveFormsModule, Validators} from '@angular/forms';
import {Router} from '@angular/router';
import {ButtonComponent} from 'src/app/components/button/button.component';
import {
    InputDirective,
    InputErrorComponent,
    InputFieldDirective,
    LabelComponent,
    PasswordInputFieldComponent
} from 'src/app/components/input/input.component';
import {LoginService} from './login.service';
import {LoaderModule} from 'src/app/private/components/loader/loader.module';
import {PrivateService} from 'src/app/private/private.service';
import {ORGANIZATION_DATA} from 'src/app/models/organisation.model';
import {SignupService} from '../signup/signup.service';
import {LicensesService} from 'src/app/services/licenses/licenses.service';
import {GoogleOAuthService} from 'src/app/services/google-oauth/google-oauth.service';
import {ConfigService} from 'src/app/services/config/config.service';
import {GeneralService} from 'src/app/services/general/general.service';

@Component({
	selector: 'app-login',
	standalone: true,
	imports: [CommonModule, ReactiveFormsModule, ButtonComponent, InputFieldDirective, InputDirective, LabelComponent, InputErrorComponent, PasswordInputFieldComponent, LoaderModule],
	templateUrl: './login.component.html',
	styleUrls: ['./login.component.scss']
})
export class LoginComponent implements OnInit, AfterViewInit {
	showLoginPassword = false;
	disableLoginBtn = false;
	loginForm: FormGroup = this.formBuilder.group({
		username: ['', Validators.required],
		password: ['', Validators.required]
	});
	isLoadingProject = false;
	isFetchingConfig = false;
	isGoogleSigningIn = false;
	isSignupEnabled = false;
	isGoogleOAuthEnabled = false;
	isSAMLEnabled = false;
	googleClientId = '';
	organisations?: ORGANIZATION_DATA[];

	constructor(
		private formBuilder: FormBuilder,
		public router: Router,
		private loginService: LoginService,
		private signupService: SignupService,
		private privateService: PrivateService,
		public licenseService: LicensesService,
		private googleOAuthService: GoogleOAuthService,
		private configService: ConfigService,
		private generalService: GeneralService
	) {}

	ngOnInit() {
		this.licenseService.setLicenses();
	}

	async ngAfterViewInit() {

		try {
			const config = await this.configService.getConfig();
			this.isGoogleOAuthEnabled = config.auth?.google_oauth?.enabled || false;
			this.isSAMLEnabled = config.auth?.saml?.enabled || false;
			this.googleClientId = config.auth?.google_oauth?.client_id || '';
			this.isSignupEnabled = config.auth?.is_signup_enabled || false;

		} catch (error) {
			console.error('Failed to get config:', error);
			this.isGoogleOAuthEnabled = false;
			this.isSAMLEnabled = false;
			this.googleClientId = '';
			this.isSignupEnabled = false;
		}


		if (this.isGoogleOAuthEnabled) {
			await this.googleOAuthService.initialize();

			// Add global error handler for Google OAuth errors
			this.setupGoogleOAuthErrorHandler();
		}
	}



	private setupGoogleOAuthErrorHandler() {
		// Listen for Google OAuth errors that might not be caught by our Promise
		window.addEventListener('error', (event) => {
			if (event.error && event.error.message) {
				const errorMessage = event.error.message;
				if (errorMessage.includes('FedCM') || errorMessage.includes('NetworkError')) {
					this.generalService.showNotification({
						message: 'Browser blocked authentication. Check browser settings.',
						style: 'error'
					});
				}
			}
		});

		// Listen for unhandled promise rejections
		window.addEventListener('unhandledrejection', (event) => {
			if (event.reason && event.reason.message) {
				const errorMessage = event.reason.message;
				if (errorMessage.includes('FedCM') || errorMessage.includes('NetworkError')) {
					this.generalService.showNotification({
						message: 'Browser blocked authentication. Check browser settings.',
						style: 'error'
					});
				}
			}
		});
	}

	async login() {
		if (this.loginForm.invalid) return this.loginForm.markAllAsTouched();

		this.disableLoginBtn = true;
		try {
			const response: any = await this.loginService.login(this.loginForm.value);

			// Check if response has the expected structure
			if (!response || !response.data) {
				throw new Error('Invalid response structure from login API - response or response.data is missing');
			}

			const lastUserId = localStorage.getItem('CONVOY_LAST_USER_ID');

            let refresh = true;
            if (lastUserId && lastUserId !== response.data.uid) {
				localStorage.clear();
                refresh = true;
			}

			localStorage.setItem('CONVOY_LAST_USER_ID', response.data.uid);
			localStorage.setItem('CONVOY_AUTH', JSON.stringify(response.data));
			localStorage.setItem('CONVOY_AUTH_TOKENS', JSON.stringify(response.data.token));

			this.isLoadingProject = true;
			// Show success notification
			this.generalService.showNotification({
				message: 'Login successful! Welcome back.',
				style: 'success'
			});

			await this.getOrganisations();
			await this.router.navigateByUrl('/');
			if (refresh)
				window.location.reload();
		} catch (error: any) {
			console.error('Login failed:', error);

			// HttpService now rejects with the error message directly
			let errorMessage = typeof error === 'string' ? error : (error?.message || error?.error?.message || 'Login failed');

			this.generalService.showNotification({
				message: errorMessage,
				style: 'error'
			});

			this.disableLoginBtn = false;
		}
	}

	async getOrganisations() {
		try {
			await this.privateService.getOrganizations({ refresh: true });
			return;
		} catch (error) {
			return error;
		}
	}

	async loginWithSAML() {
		localStorage.setItem('AUTH_TYPE', 'login');

		try {
			const res = await this.loginService.loginWithSaml();

			const { redirectUrl } = res.data;
			window.open(redirectUrl);
		} catch (error: any) {
			console.error('SAML login failed:', error);

			let errorMessage = 'SAML login failed';
			if (error?.message) {
				errorMessage = error.message;
			} else if (error?.error?.message) {
				errorMessage = error.error.message;
			}

			this.generalService.showNotification({
				message: errorMessage,
				style: 'error'
			});
		}
	}

    async loginWithGoogle() {
        try {
            if (!this.isGoogleOAuthEnabled) {
                this.generalService.showNotification({
                    message: 'Google OAuth is disabled',
                    style: 'error'
                });
                return;
            }

            if (!this.googleOAuthService.isReady()) {
                this.generalService.showNotification({
                    message: 'Google OAuth not initialized',
                    style: 'error'
                });
                return;
            }

            this.isGoogleSigningIn = true;

            const result = await this.googleOAuthService.signIn();

            if (result.credential) {
                const response = await this.loginService.loginWithGoogleToken(result.credential);

                if (response.data) {
                    if (response.data.needs_setup) {
                        localStorage.setItem('GOOGLE_OAUTH_ID_TOKEN', result.credential);
                        localStorage.setItem('GOOGLE_OAUTH_USER_INFO', JSON.stringify({
                            name: response.data.first_name + ' ' + response.data.last_name,
                            email: response.data.email,
                            picture: response.data.picture
                        }));

                        await this.router.navigateByUrl('/google-oauth-setup');
                        this.isGoogleSigningIn = false;
                        return;
                    }

                    // Check if this is a different user
					const lastUserId = localStorage.getItem('CONVOY_LAST_USER_ID');
					let refresh = false;
					if (lastUserId && lastUserId !== response.data.uid) {
						localStorage.clear();
						refresh = true;
					}

					// Show success notification
					this.generalService.showNotification({
						message: 'Google login successful! Welcome back.',
						style: 'success'
					});

					localStorage.setItem('CONVOY_LAST_USER_ID', response.data.uid);
					localStorage.setItem('CONVOY_AUTH', JSON.stringify(response.data));
					localStorage.setItem('CONVOY_AUTH_TOKENS', JSON.stringify(response.data.token));

					await this.getOrganisations();
					await this.router.navigateByUrl('/');
					if (refresh) {
						window.location.reload();
					}
                }
            }
        } catch (error: any) {
            console.error('Google login failed:', error);

            // Show user-friendly error messages
            let errorMessage = 'Google login failed';

            if (error?.message) {
                if (error.message.includes('FedCM') || error.message.includes('NetworkError')) {
                    errorMessage = 'Browser blocked authentication. Check browser settings for third-party sign-in.';
                } else if (error.message.includes('prompt was skipped')) {
                    errorMessage = 'Sign-in blocked. Check browser settings for third-party sign-in.';
                } else if (error.message.includes('prompt not displayed')) {
                    errorMessage = 'Sign-in prompt unavailable. Check browser settings.';
                } else if (error.message.includes('cancelled by user') ||
                           error.message.includes('was cancelled') ||
                           error.message.includes('prompt was dismissed') ||
                           error.message.includes('popup_closed')) {
                    // User intentionally cancelled - no need for error toast
                    return; // Exit early, no error notification
                } else if (error.message.includes('access_denied')) {
                    errorMessage = 'Access denied. Please try again.';
                } else {
                    errorMessage = error.message;
                }
            }

            this.generalService.showNotification({
                message: errorMessage,
                style: 'error'
            });
        } finally {
            this.isGoogleSigningIn = false;
        }
    }
}
