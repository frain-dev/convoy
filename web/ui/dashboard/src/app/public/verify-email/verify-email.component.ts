import { Component, OnInit } from '@angular/core';

import { ActivatedRoute, RouterModule } from '@angular/router';
import { VerifyEmailService } from './verify-email.service';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { LoaderModule } from 'src/app/private/components/loader/loader.module';

@Component({
    selector: 'convoy-verify-email',
    imports: [RouterModule, ButtonComponent, LoaderModule],
    templateUrl: './verify-email.component.html',
    styleUrls: ['./verify-email.component.scss']
})
export class VerifyEmailComponent implements OnInit {
	token = this.route.snapshot.queryParams['verification-token'];
	loading = true;
	showError = false;

	constructor(private route: ActivatedRoute, private verifyEmailService: VerifyEmailService) {}

	ngOnInit() {
		this.verifyEmail();
	}

	async verifyEmail() {
		this.showError = false;
		try {
			await this.verifyEmailService.verifyEmail(this.token);
			this.patchAuthEmailVerified();
			this.loading = false;
		} catch {
			this.loading = false;
			this.showError = true;
		}
	}

	// Keep CONVOY_AUTH in sync so dashboard surfaces (trial modal, verify chip)
	// do not keep showing unverified after a successful verify redirect.
	private patchAuthEmailVerified(): void {
		try {
			const raw = localStorage.getItem('CONVOY_AUTH');
			if (!raw) return;
			const auth = JSON.parse(raw);
			auth.email_verified = true;
			localStorage.setItem('CONVOY_AUTH', JSON.stringify(auth));
		} catch {
			// ignore
		}
	}
}
