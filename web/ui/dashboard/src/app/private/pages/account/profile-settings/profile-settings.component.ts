import { Component, OnInit } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { PrivateService } from 'src/app/private/private.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { AccountService } from '../account.service';

@Component({
	selector: 'profile-settings',
	templateUrl: './profile-settings.component.html',
	styleUrls: ['./profile-settings.component.scss']
})
export class ProfileSettingsComponent implements OnInit {
	isSavingUserDetails = false;
	isUpdatingPassword = false;
	isFetchingUserDetails = false;
	userId!: string;
	passwordToggle = { oldPassword: false, newPassword: false, confirmPassword: false };
	editBasicInfoForm: FormGroup = this.formBuilder.group({
		first_name: ['', Validators.required],
		last_name: ['', Validators.required],
		email: ['', Validators.compose([Validators.required, Validators.email])]
	});

	constructor(private formBuilder: FormBuilder, private router: Router, private accountService: AccountService, private privateService:PrivateService, private generalService: GeneralService) {}

	ngOnInit() {
		this.getAuthDetails();
	}

	getAuthDetails() {
		const authDetails = localStorage.getItem('CONVOY_AUTH');
		if (authDetails && authDetails !== 'undefined') {
			const userId = JSON.parse(authDetails)?.uid;
			this.getUserDetails(userId);
		} else {
			this.router.navigateByUrl('/login');
		}
	}

	async getUserDetails(userId: string) {
		this.isFetchingUserDetails = true;

		try {
			const response = await this.privateService.getUserDetails({ userId: userId });
			this.userId = response.data?.uid;
			this.editBasicInfoForm.patchValue({
				first_name: response.data?.first_name,
				last_name: response.data?.last_name,
				email: response.data?.email
			});
			this.isFetchingUserDetails = false;
		} catch {
			this.isFetchingUserDetails = false;
		}
	}

	async editBasicUserInfo() {
		if (this.editBasicInfoForm.invalid) return this.editBasicInfoForm.markAllAsTouched();
		this.isSavingUserDetails = true;
		try {
			const response = await this.accountService.editBasicInfo({ userId: this.userId, body: this.editBasicInfoForm.value });
			this.generalService.showNotification({ style: 'success', message: 'Changes saved successfully!' });
			this.getUserDetails(this.userId);
			this.isSavingUserDetails = false;
		} catch {
			this.isSavingUserDetails = false;
		}
	}
}
