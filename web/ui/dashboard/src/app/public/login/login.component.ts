import { CommonModule } from '@angular/common';
import { Component, OnInit } from '@angular/core';
import { FormBuilder, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent, PasswordInputFieldComponent } from 'src/app/components/input/input.component';
import { LoginService } from './login.service';

@Component({
	selector: 'app-login',
	standalone: true,
	imports: [CommonModule, ReactiveFormsModule, ButtonComponent, InputFieldDirective, InputDirective, LabelComponent, InputErrorComponent, PasswordInputFieldComponent],
	templateUrl: './login.component.html',
	styleUrls: ['./login.component.scss']
})
export class LoginComponent implements OnInit {
	showLoginPassword = false;
	disableLoginBtn = false;
	loginForm: FormGroup = this.formBuilder.group({
		username: ['', Validators.required],
		password: ['', Validators.required]
	});

	constructor(private formBuilder: FormBuilder, public router: Router, private loginService: LoginService) {}

	ngOnInit(): void {}

	async login() {
		if (this.loginForm.invalid) return this.loginForm.markAsTouched();

		this.disableLoginBtn = true;
		try {
			const response: any = await this.loginService.login(this.loginForm.value);
			localStorage.setItem('CONVOY_AUTH', JSON.stringify(response.data));
			localStorage.setItem('CONVOY_AUTH_TOKENS', JSON.stringify(response.data.token));

			// get previous location in localstorage
			const lastLoacation = localStorage.getItem('CONVOY_LAST_AUTH_LOCATION');

			lastLoacation ? (location.href = lastLoacation) : this.router.navigateByUrl('/');
		} catch {
			this.disableLoginBtn = false;
		}
	}
}
