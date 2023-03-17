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

	constructor(private formBuilder: FormBuilder, private router: Router, private accountService: AccountService, private privateService: PrivateService, private generalService: GeneralService) {}

	ngOnInit() {
		this.getAuthDetails();
	}

	getAuthDetails() {
		const authDetails = localStorage.getItem('CONVOY_AUTH');
		if (authDetails && authDetails !== 'undefined') {
			const userDetails = JSON.parse(authDetails);
			this.editBasicInfoForm.patchValue({
				first_name: userDetails?.first_name,
				last_name: userDetails?.last_name,
				email: userDetails?.email
			});
			this.userId = userDetails?.uid;
		} else {
			this.router.navigateByUrl('/login');
		}
	}

	async editBasicUserInfo() {
		if (this.editBasicInfoForm.invalid) return this.editBasicInfoForm.markAllAsTouched();
		this.isSavingUserDetails = true;

		try {
			const response = await this.accountService.editBasicInfo({ userId: this.userId, body: this.editBasicInfoForm.value });

			this.generalService.showNotification({ style: 'success', message: 'Changes saved successfully!' });
			localStorage.setItem('CONVOY_AUTH', JSON.stringify(response.data));

			this.isSavingUserDetails = false;
		} catch {
			this.isSavingUserDetails = false;
		}
	}
}
