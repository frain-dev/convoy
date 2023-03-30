import { Component, OnInit } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { GeneralService } from 'src/app/services/general/general.service';
import { AccountService } from '../account.service';

@Component({
	selector: 'security-settings',
	templateUrl: './security-settings.component.html',
	styleUrls: ['./security-settings.component.scss']
})
export class SecuritySettingsComponent implements OnInit {
	isUpdatingPassword = false;
	passwordToggle = { oldPassword: false, newPassword: false, confirmPassword: false };
	changePasswordForm: FormGroup = this.formBuilder.group({
		current_password: ['', Validators.required],
		password: ['', Validators.required],
		password_confirmation: ['', Validators.required]
	});

	constructor(private formBuilder: FormBuilder, private accountService: AccountService, private generalService: GeneralService) {}

	ngOnInit(): void {}

	async changePassword() {
		if (this.changePasswordForm.invalid) return this.changePasswordForm.markAllAsTouched();
		this.isUpdatingPassword = true;

		try {
			let userData = localStorage.getItem('CONVOY_AUTH');
			if (userData) userData = JSON.parse(userData)?.uid;
			const response = await this.accountService.changePassword({ userId: userData || '', body: this.changePasswordForm.value });
			this.generalService.showNotification({ style: 'success', message: response.message });
			this.changePasswordForm.reset();
			this.isUpdatingPassword = false;
		} catch {
			this.isUpdatingPassword = false;
		}
	}

	checkPassword(): boolean {
		const newPassword = this.changePasswordForm.value.password;
		const confirmPassword = this.changePasswordForm.value.password_confirmation;
		if (newPassword === confirmPassword) return true;
		return false;
	}
}
