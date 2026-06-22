
import {Component, OnInit} from '@angular/core';
import {FormBuilder, FormGroup, ReactiveFormsModule, Validators} from '@angular/forms';
import {Router} from '@angular/router';
import {ButtonComponent} from 'src/app/components/button/button.component';
import {
    InputDirective,
    InputErrorComponent,
    InputFieldDirective,
    LabelComponent
} from 'src/app/components/input/input.component';
import {LoaderModule} from 'src/app/private/components/loader/loader.module';
import {HubspotService} from 'src/app/services/hubspot/hubspot.service';
import {SignupService} from './signup.service';
import {LicensesService} from 'src/app/services/licenses/licenses.service';
import {ConfigService} from 'src/app/services/config/config.service';
import {GoogleOAuthService} from 'src/app/services/google-oauth/google-oauth.service';
import {LoginService} from '../login/login.service';
import {PrivateService} from 'src/app/private/private.service';
import {GeneralService} from 'src/app/services/general/general.service';

@Component({
    selector: 'convoy-signup',
    imports: [ReactiveFormsModule, ButtonComponent, InputErrorComponent, InputDirective, LabelComponent, InputFieldDirective, LoaderModule],
    templateUrl: './signup.component.html',
    styleUrls: ['./signup.component.scss']
})
export class SignupComponent implements OnInit {
	showSignupPassword = false;
	disableSignupBtn = false;
	isFetchingConfig = false;
	isGoogleOAuthEnabled = false;
	isGoogleSigningIn = false;
	authConfigLoaded = false;
	signupForm: FormGroup = this.formBuilder.group({
		email: ['', Validators.required],
		first_name: ['', Validators.required],
		last_name: ['', Validators.required],
		password: ['', Validators.required],
		org_name: ['', Validators.required]
	});

	constructor(
		private formBuilder: FormBuilder,
		private signupService: SignupService,
		public router: Router,
		private hubspotService: HubspotService,
		private licenseService: LicensesService,
		private configService: ConfigService,
		private googleOAuthService: GoogleOAuthService,
		private loginService: LoginService,
		private privateService: PrivateService,
		private generalService: GeneralService
	) {}

	async ngOnInit(): Promise<void> {
		await Promise.all([this.licenseService.setLicenses(true), this.checkGoogleOAuthConfig()]);

		if (!this.licenseService.hasInstanceLicense('user_limit')) this.router.navigateByUrl('/login');
	}

	async signup() {
		if (this.signupForm.invalid) return this.signupForm.markAllAsTouched();

		this.disableSignupBtn = true;
		try {
			const response: any = await this.signupService.signup(this.signupForm.value);
			localStorage.setItem('CONVOY_AUTH', JSON.stringify(response.data));
			localStorage.setItem('CONVOY_AUTH_TOKENS', JSON.stringify(response.data.token));

			// Clear org/project and licenses so private app uses the new user's org, not a previous session
			localStorage.removeItem('CONVOY_ORG');
			localStorage.removeItem('CONVOY_PROJECT');
			this.licenseService.clearLicenses();

			if (window.location.hostname === 'dashboard.getconvoy.io') await this.hubspotService.sendWelcomeEmail({ email: this.signupForm.value.email, firstname: this.signupForm.value.first_name, lastname: this.signupForm.value.last_name });

			this.router.navigateByUrl('/projects');
			this.disableSignupBtn = false;
		} catch {
			this.disableSignupBtn = false;
		}
	}

	private async checkGoogleOAuthConfig() {
		try {
			const config = await this.configService.getConfig();
			this.isGoogleOAuthEnabled = config.auth?.google_oauth?.enabled || false;
		} catch (error) {
			this.isGoogleOAuthEnabled = false;
		} finally {
			this.authConfigLoaded = true;
		}

		if (this.isGoogleOAuthEnabled) {
			await this.googleOAuthService.initialize();
		}
	}

	async signUpWithSSO() {
		localStorage.setItem('AUTH_TYPE', 'signup');

		try {
			const res = await this.signupService.signUpWithSSO();
			const { redirectUrl } = res.data;
			window.open(redirectUrl, '_blank');
		} catch (error) {
			throw error;
		}
	}

	private clearCachedSession(previousUserId?: string) {
		localStorage.removeItem('CONVOY_LAST_USER_ID');
		localStorage.removeItem('CONVOY_AUTH');
		localStorage.removeItem('CONVOY_AUTH_TOKENS');
		localStorage.removeItem('CONVOY_ORG');
		localStorage.removeItem('CONVOY_PROJECT');
		this.licenseService.clearLicenses();

		if (previousUserId) {
			this.privateService.clearCache(true, previousUserId);
		}
	}

	async signUpWithGoogle() {
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
			if (!result.credential) return;

			const response = await this.loginService.loginWithGoogleToken(result.credential);
			if (!response.data) return;

			if (response.data.needs_setup) {
				this.clearCachedSession(localStorage.getItem('CONVOY_LAST_USER_ID') || undefined);
				localStorage.setItem('AUTH_TYPE', 'signup');
				localStorage.setItem('GOOGLE_OAUTH_ID_TOKEN', result.credential);
				localStorage.setItem('GOOGLE_OAUTH_USER_INFO', JSON.stringify({
					name: response.data.first_name + ' ' + response.data.last_name,
					email: response.data.email,
					picture: response.data.picture
				}));

				await this.router.navigateByUrl('/google-oauth-setup');
				return;
			}

			const lastUserId = localStorage.getItem('CONVOY_LAST_USER_ID');
			if (!lastUserId || lastUserId !== response.data.uid) {
				this.clearCachedSession(lastUserId || undefined);
			}

			localStorage.setItem('CONVOY_LAST_USER_ID', response.data.uid);
			localStorage.setItem('CONVOY_AUTH', JSON.stringify(response.data));
			localStorage.setItem('CONVOY_AUTH_TOKENS', JSON.stringify(response.data.token));

			this.generalService.showNotification({
				message: 'Google signup successful! Welcome.',
				style: 'success'
			});

			try {
				await this.privateService.getOrganizations({ refresh: true });
			} catch {
				// Google auth already succeeded; org refresh is best effort and should not surface as signup failure.
			}

			await this.router.navigateByUrl('/');
		} catch (error: any) {
			const rawErrorMessage = typeof error === 'string' ? error : (error?.message || error?.error?.message);
			let errorMessage = rawErrorMessage || 'Google signup failed';

			if (rawErrorMessage) {
				if (rawErrorMessage.includes('FedCM') || rawErrorMessage.includes('NetworkError')) {
					errorMessage = 'Browser blocked authentication. Check browser settings for third-party sign-in.';
				} else if (rawErrorMessage.includes('prompt was skipped')) {
					errorMessage = 'Sign-in blocked. Check browser settings for third-party sign-in.';
				} else if (rawErrorMessage.includes('prompt not displayed')) {
					errorMessage = 'Sign-in prompt unavailable. Check browser settings.';
				} else if (rawErrorMessage.includes('cancelled by user') ||
					rawErrorMessage.includes('was cancelled') ||
					rawErrorMessage.includes('prompt was dismissed') ||
					rawErrorMessage.includes('popup_closed')) {
					return;
				} else if (rawErrorMessage.includes('access_denied')) {
					errorMessage = 'Access denied. Please try again.';
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
