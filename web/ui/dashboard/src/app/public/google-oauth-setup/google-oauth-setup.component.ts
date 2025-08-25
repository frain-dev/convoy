import {CommonModule} from '@angular/common';
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
import {GeneralService} from 'src/app/services/general/general.service';
import {GoogleOAuthSetupService} from './google-oauth-setup.service';

@Component({
	selector: 'app-google-oauth-setup',
	standalone: true,
	imports: [CommonModule, ReactiveFormsModule, ButtonComponent, InputFieldDirective, InputErrorComponent, InputDirective, LabelComponent],
	templateUrl: './google-oauth-setup.component.html',
	styleUrls: ['./google-oauth-setup.component.scss']
})
export class GoogleOAuthSetupComponent implements OnInit {
	loading = false;
	userInfo: any = null;
	setupForm: FormGroup = this.formBuilder.group({
		business_name: ['', Validators.required]
	});

	constructor(
		private formBuilder: FormBuilder,
		private googleOAuthSetupService: GoogleOAuthSetupService,
		private router: Router,
		private generalService: GeneralService
	) {}

	ngOnInit(): void {
		if (!this.hasValidSetupData()) {
			this.router.navigateByUrl('/login');
			return;
		}
		this.getUserInfo();
	}

	getUserInfo() {
		try {
			const userData = localStorage.getItem('GOOGLE_OAUTH_USER_INFO');
			if (userData) {
				this.userInfo = JSON.parse(userData);
			}
		} catch (error) {
			console.error('Failed to get user info:', error);
		}
	}

	async completeSetup(): Promise<void> {
		if (this.setupForm.invalid) {
			this.setupForm.markAllAsTouched();
			return;
		}

		this.loading = true;
		try {
			const idToken = localStorage.getItem('GOOGLE_OAUTH_ID_TOKEN');
			const businessName = this.setupForm.get('business_name')?.value;

			if (!idToken || !businessName) {
				throw new Error('Missing required setup data');
			}

			const response = await this.googleOAuthSetupService.completeSetup(idToken, businessName);

			if (response.data) {
				this.generalService.showNotification({
					message: 'Setup completed successfully! Welcome to Convoy.',
					style: 'success'
				});
				localStorage.setItem('CONVOY_AUTH', JSON.stringify(response.data));
				localStorage.setItem('CONVOY_AUTH_TOKENS', JSON.stringify(response.data.token));
				localStorage.setItem('CONVOY_LAST_USER_ID', response.data.uid);
				localStorage.removeItem('GOOGLE_OAUTH_USER_INFO');
				localStorage.removeItem('GOOGLE_OAUTH_ID_TOKEN');
				await this.router.navigateByUrl('/');
			}
		} catch (error: any) {
			this.generalService.showNotification({
				message: error.error?.message || 'Setup failed. Please try again.',
				style: 'error'
			});
		} finally {
			this.loading = false;
		}
	}

	private hasValidSetupData(): boolean {
		const hasIdToken = localStorage.getItem('GOOGLE_OAUTH_ID_TOKEN');
		const hasUserInfo = localStorage.getItem('GOOGLE_OAUTH_USER_INFO');
		return !!(hasIdToken && hasUserInfo);
	}
}
