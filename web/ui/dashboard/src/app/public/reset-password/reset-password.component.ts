import { CommonModule } from '@angular/common';
import { Component, OnInit } from '@angular/core';
import { FormBuilder, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent } from 'src/app/components/input/input.component';
import { GeneralService } from 'src/app/services/general/general.service';
import { ResetPasswordService } from './reset-password.service';

@Component({
	selector: 'app-reset-password',
	standalone: true,
	imports: [CommonModule, ReactiveFormsModule, ButtonComponent, InputErrorComponent, InputDirective, LabelComponent, InputFieldDirective],
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
	activePage: 'resetPassword' | 'success' = 'resetPassword';
	token!: string;

	constructor(private formBuilder: FormBuilder, private resetPasswordService: ResetPasswordService, private route: ActivatedRoute, public router: Router, private generalService: GeneralService) {}

	ngOnInit() {
		this.token = this.route.snapshot.queryParams?.token;
	}

	async resetPassword() {
		if (this.resetPasswordForm.invalid) return this.resetPasswordForm.markAsTouched();

		this.resetingPassword = true;
		try {
			const response = await this.resetPasswordService.resetPassword({ token: this.token, body: this.resetPasswordForm.value });
			this.resetingPassword = false;
			this.generalService.showNotification({ style: 'success', message: response.message });
			this.activePage = 'success';
		} catch {
			this.resetingPassword = false;
		}
	}
}
