import { CommonModule } from '@angular/common';
import { Component, OnInit } from '@angular/core';
import { FormBuilder, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent } from 'src/app/components/input/input.component';
import { LoaderModule } from 'src/app/private/components/loader/loader.module';
import { HubspotService } from 'src/app/services/hubspot/hubspot.service';
import { SignupService } from './signup.service';

@Component({
	selector: 'convoy-signup',
	standalone: true,
	imports: [CommonModule, ReactiveFormsModule, ButtonComponent, InputErrorComponent, InputDirective, LabelComponent, InputFieldDirective, LoaderModule],
	templateUrl: './signup.component.html',
	styleUrls: ['./signup.component.scss']
})
export class SignupComponent implements OnInit {
	showSignupPassword = false;
	disableSignupBtn = false;
	isFetchingConfig = false;
	signupForm: FormGroup = this.formBuilder.group({
		email: ['', Validators.required],
		first_name: ['', Validators.required],
		last_name: ['', Validators.required],
		password: ['', Validators.required],
		org_name: ['', Validators.required]
	});

	constructor(private formBuilder: FormBuilder, private signupService: SignupService, public router: Router, private hubspotService: HubspotService) {}

	ngOnInit(): void {}

	async signup() {
		if (this.signupForm.invalid) return this.signupForm.markAllAsTouched();

		this.disableSignupBtn = true;
		try {
			const response: any = await this.signupService.signup(this.signupForm.value);
			localStorage.setItem('CONVOY_AUTH', JSON.stringify(response.data));
			localStorage.setItem('CONVOY_AUTH_TOKENS', JSON.stringify(response.data.token));

			if (window.location.hostname === 'dashboard.getconvoy.io') await this.hubspotService.sendWelcomeEmail({ email: this.signupForm.value.email, firstname: this.signupForm.value.first_name, lastname: this.signupForm.value.last_name });

			this.router.navigateByUrl('/get-started');
			this.disableSignupBtn = false;
		} catch {
			this.disableSignupBtn = false;
		}
	}

	async getSignUpConfig() {
		this.isFetchingConfig = true;
		try {
			const response = await this.signupService.getSignupConfig();
			const isSignupEnabled = response.data;
			if (!isSignupEnabled) this.router.navigateByUrl('/login');
			this.isFetchingConfig = false;
		} catch (error) {
			this.isFetchingConfig = false;
			return error;
		}
	}
}
