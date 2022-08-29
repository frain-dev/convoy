import { Component, OnInit } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { SignupService } from './signup.service';

@Component({
	selector: 'convoy-signup',
	templateUrl: './signup.component.html',
	styleUrls: ['./signup.component.scss']
})
export class SignupComponent implements OnInit {
	showSignupPassword = false;
	disableSignupBtn = false;
	signupForm: FormGroup = this.formBuilder.group({
		email: ['', Validators.required],
		first_name: ['', Validators.required],
		last_name: ['', Validators.required],
		password: ['', Validators.required],
		org_name: ['', Validators.required]
	});

	constructor(private formBuilder: FormBuilder, private signupService: SignupService, private router: Router) {}

	ngOnInit(): void {}

	async signup() {
		if (this.signupForm.invalid) return this.signupForm.markAsTouched();

		this.disableSignupBtn = true;
		try {
			const response: any = await this.signupService.signup(this.signupForm.value);
			localStorage.setItem('CONVOY_AUTH', JSON.stringify(response.data));
			this.router.navigateByUrl('/');
			this.disableSignupBtn = false;
		} catch {
			this.disableSignupBtn = false;
		}
	}
}
