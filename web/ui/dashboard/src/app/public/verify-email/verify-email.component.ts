import { Component, OnInit } from '@angular/core';

import { ActivatedRoute, RouterModule } from '@angular/router';
import { VerifyEmailService } from './verify-email.service';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { LoaderModule } from 'src/app/private/components/loader/loader.module';
import { PrivateService } from 'src/app/private/private.service';

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

	constructor(private route: ActivatedRoute, private verifyEmailService: VerifyEmailService, private privateService: PrivateService) {}

	ngOnInit() {
		this.verifyEmail();
	}

	async verifyEmail() {
		this.showError = false;
		try {
			await this.verifyEmailService.verifyEmail(this.token);
			// Sync CONVOY_AUTH and drop the cached profile so dashboard surfaces
			// (verify chip, trial modal) refetch instead of showing stale state.
			this.privateService.setAuthEmailVerified(true);
			this.loading = false;
		} catch {
			this.loading = false;
			this.showError = true;
		}
	}
}
