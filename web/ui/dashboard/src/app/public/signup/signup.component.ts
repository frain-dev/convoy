import { CommonModule } from '@angular/common';
import { Component, OnInit } from '@angular/core';
import { FormBuilder, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent } from 'src/app/components/input/input.component';
import { HubspotService } from 'src/app/services/hubspot/hubspot.service';
import { SignupService } from './signup.service';

@Component({
	selector: 'convoy-signup',
	standalone: true,
	imports: [CommonModule, ReactiveFormsModule, ButtonComponent, InputErrorComponent, InputDirective, LabelComponent, InputFieldDirective],
	templateUrl: './signup.component.html',
	styleUrls: ['./signup.component.scss']
})
export class SignupComponent implements OnInit {
	showSignupPassword = false;
	disableSignupBtn = false;
	isSignUpDone = false;
	signupForm: FormGroup = this.formBuilder.group({
		email: ['', Validators.required],
		first_name: ['', Validators.required],
		last_name: ['', Validators.required],
		password: ['', Validators.required],
		org_name: ['', Validators.required]
	});

	constructor(private formBuilder: FormBuilder, private signupService: SignupService, public router: Router, public hubspotService: HubspotService) {}

	ngOnInit(): void {}

	async signup() {
		if (this.signupForm.invalid) return this.signupForm.markAsTouched();

		this.disableSignupBtn = true;
		try {
			const response: any = await this.signupService.signup(this.signupForm.value);
			localStorage.setItem('CONVOY_AUTH', JSON.stringify(response.data));

			if (window.location.hostname === 'dashboard.getconvoy.io') this.hubspotService.sendWelcomeEmail({ email: this.signupForm.value.email, firstname: this.signupForm.value.first_name, lastname: this.signupForm.value.last_name });

			this.isSignUpDone = true;
			this.disableSignupBtn = false;
		} catch {
			this.disableSignupBtn = false;
		}
	}
}
