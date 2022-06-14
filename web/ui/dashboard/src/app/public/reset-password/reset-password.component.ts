import { Component, OnInit } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { ActivatedRoute } from '@angular/router';
import { GeneralService } from 'src/app/services/general/general.service';
import { ResetPasswordService } from './reset-password.service';

@Component({
	selector: 'app-reset-password',
	templateUrl: './reset-password.component.html',
	styleUrls: ['./reset-password.component.scss']
})
export class ResetPasswordComponent implements OnInit {
	resetPasswordForm: FormGroup = this.formBuilder.group({
		password: ['', Validators.required],
		password_confirmation: ['', Validators.required]
	});
	showPassword: boolean = false;
	showCofirmPassword: boolean = false;
	resetingPassword: boolean = false;
	activePage: 'reset-password' | 'success' = 'reset-password';
	token!: string;

	constructor(private formBuilder: FormBuilder, private resetPasswordService: ResetPasswordService, private route: ActivatedRoute, private generalService: GeneralService) {}

	ngOnInit() {
		this.token = this.route.snapshot.queryParams?.token;
	}

	async resetPassword() {
		this.resetingPassword = true;
		try {
			const response = await this.resetPasswordService.resetPassword({ token: this.token, body: this.resetPasswordForm.value });
			this.resetingPassword = false;
			if (response.status === true) {
				this.generalService.showNotification({ style: 'success', message: response.message });
				this.activePage = 'success';
			}
		} catch {
			this.resetingPassword = false;
		}
	}
}
