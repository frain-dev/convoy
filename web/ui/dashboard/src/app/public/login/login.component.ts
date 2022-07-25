import { Component, OnInit } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { LoginService } from './login.service';

@Component({
	selector: 'app-login',
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

	constructor(private formBuilder: FormBuilder, private router: Router, private loginService: LoginService) {}

	ngOnInit(): void {}

	async login() {
		if (this.loginForm.invalid) return this.loginForm.markAsTouched();

		this.disableLoginBtn = true;
		try {
			const response: any = await this.loginService.login(this.loginForm.value);
			localStorage.setItem('CONVOY_AUTH', JSON.stringify(response.data));
			this.router.navigateByUrl('/');
			this.disableLoginBtn = false;
		} catch {
			this.disableLoginBtn = false;
		}
	}
}
