import { CommonModule } from '@angular/common';
import { Component, OnInit } from '@angular/core';
import { FormBuilder, FormControl, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { InputComponent, InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent } from 'src/app/components/input/input.component';
import { GeneralService } from 'src/app/services/general/general.service';
import { ForgotPasswordService } from './forgot-password.service';

@Component({
	selector: 'app-forgot-password',
	standalone: true,
	imports: [CommonModule, ReactiveFormsModule, ButtonComponent, InputComponent, InputErrorComponent, InputDirective, LabelComponent, InputFieldDirective],
	templateUrl: './forgot-password.component.html',
	styleUrls: ['./forgot-password.component.scss']
})
export class ForgotPasswordComponent implements OnInit {
	forgotPasswordForm: FormGroup = this.formBuilder.group({
		email: ['', Validators.compose([Validators.required, Validators.email])]
	});
	activeState: 'resetPassword' | 'instructionSent' = 'resetPassword';
	loading: boolean = false;
	constructor(private formBuilder: FormBuilder, private forgotPasswordService: ForgotPasswordService, private generalService: GeneralService, public router: Router) {}

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
