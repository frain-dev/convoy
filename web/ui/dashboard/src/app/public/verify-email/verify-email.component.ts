import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, RouterModule } from '@angular/router';
import { VerifyEmailService } from './verify-email.service';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { LoaderModule } from 'src/app/private/components/loader/loader.module';

@Component({
	selector: 'convoy-verify-email',
	standalone: true,
	imports: [CommonModule, RouterModule, ButtonComponent, LoaderModule],
	templateUrl: './verify-email.component.html',
	styleUrls: ['./verify-email.component.scss']
})
export class VerifyEmailComponent implements OnInit {
	token = this.route.snapshot.queryParams.token;
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
			this.loading = false;
		} catch {
			this.loading = false;
			this.showError = true;
		}
	}
}
