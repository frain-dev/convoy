import { Component, OnInit } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { HttpService } from 'src/app/services/http/http.service';

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

	constructor(private formBuilder: FormBuilder, private router: Router, private httpService: HttpService) {}

	ngOnInit(): void {}

	async login() {
		try {
			const loginResponse = await this.httpService.request({ method: 'post', url: '/auth/login', body: this.loginForm.value });
			localStorage.setItem('CONVOY_AUTH', JSON.stringify(loginResponse.data));
			this.router.navigateByUrl('dashboard');
		} catch (error) {}
	}
}
