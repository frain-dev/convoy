import { Component, OnInit } from '@angular/core';
import { FormBuilder, FormControl, FormGroup, Validators } from '@angular/forms';
import { GeneralService } from 'src/app/services/general/general.service';
import { ForgotPasswordService } from './forgot-password.service';

@Component({
	selector: 'app-forgot-password',
	templateUrl: './forgot-password.component.html',
	styleUrls: ['./forgot-password.component.scss']
})
export class ForgotPasswordComponent implements OnInit {
	forgotPasswordForm: FormGroup = this.formBuilder.group({
		email: ['', Validators.compose([Validators.required, Validators.email])]
	});
	activeState: 'resetPassword' | 'instructionSent' = 'resetPassword';
	loading: boolean = false;
	constructor(private formBuilder: FormBuilder, private forgotPasswordService: ForgotPasswordService, private generalService: GeneralService) {}

	ngOnInit(): void {}

	async resetPassword() {
		if (this.forgotPasswordForm.invalid) {
			(<any>Object).values(this.forgotPasswordForm.controls).forEach((control: FormControl) => {
				control?.markAsTouched();
			});
			return;
		}

		this.loading = true;
		try {
			const response = await this.forgotPasswordService.forgotPassword(this.forgotPasswordForm.value);
			this.generalService.showNotification({ style: 'success', message: response.message });
			this.activeState = 'instructionSent';
			this.loading = false;
		} catch {
			this.loading = false;
		}
	}
}
