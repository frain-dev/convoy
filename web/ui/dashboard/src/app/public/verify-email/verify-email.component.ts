import { Component, OnInit } from '@angular/core';

import { ActivatedRoute, Router, RouterModule } from '@angular/router';
import { VerifyEmailService } from './verify-email.service';
import { PrivateService } from 'src/app/private/private.service';
import { AuthShellComponent } from 'src/app/public/components/auth-shell/auth-shell.component';
import { GeneralService } from 'src/app/services/general/general.service';

const EMAIL_VERIFIED_FLAG = 'convoy_email_verified';

@Component({
	selector: 'convoy-verify-email',
	imports: [RouterModule, AuthShellComponent],
	templateUrl: './verify-email.component.html',
	styleUrls: ['./verify-email.component.scss']
})
export class VerifyEmailComponent implements OnInit {
	token = this.route.snapshot.queryParams['verification-token'];
	loading = true;
	showError = false;
	isResendingEmail = false;

	constructor(
		private route: ActivatedRoute,
		private router: Router,
		private verifyEmailService: VerifyEmailService,
		private privateService: PrivateService,
		private generalService: GeneralService
	) {}

	ngOnInit() {
		this.verifyEmail();
	}

	async verifyEmail() {
		this.showError = false;

		if (!this.token) {
			this.loading = false;
			// Refresh after a successful verify strips the token; restore success from the flag.
			if (sessionStorage.getItem(EMAIL_VERIFIED_FLAG) === '1') {
				sessionStorage.removeItem(EMAIL_VERIFIED_FLAG);
				this.showError = false;
				return;
			}
			this.showError = true;
			return;
		}

		try {
			await this.verifyEmailService.verifyEmail(this.token);
			// Sync CONVOY_AUTH and drop the cached profile so dashboard surfaces
			// (verify chip, trial modal) refetch instead of showing stale state.
			this.privateService.setAuthEmailVerified(true);
			sessionStorage.setItem(EMAIL_VERIFIED_FLAG, '1');
			// Drop the consumed token from the URL so a refresh does not re-verify
			// with a cleared token and land in a stuck error/resend loop.
			this.token = '';
			await this.router.navigate([], { relativeTo: this.route, queryParams: {}, replaceUrl: true });
			this.loading = false;
		} catch {
			this.loading = false;
			this.showError = true;
		}
	}

	async resendVerificationEmail() {
		if (this.isResendingEmail) return;

		if (!this.token) {
			this.generalService.showNotification({
				message: 'This link is no longer valid. Please log in to request a new email.',
				style: 'error'
			});
			return;
		}

		this.isResendingEmail = true;
		try {
			const response = await this.verifyEmailService.resendVerificationEmail(this.token);
			this.generalService.showNotification({ message: response.message, style: 'success' });
			await this.router.navigateByUrl('/login');
		} catch {
			// Token was already consumed or invalid — clear it so Resend disables
			// and the user can leave via Back to login.
			this.token = '';
		} finally {
			this.isResendingEmail = false;
		}
	}
}
