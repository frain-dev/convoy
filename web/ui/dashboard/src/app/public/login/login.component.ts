import { Component, OnInit } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { Router } from '@angular/router';

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

	constructor(private formBuilder: FormBuilder, private router: Router) {}

	ngOnInit(): void {}

	async login() {
		localStorage.setItem('CONVOY_AUTH', JSON.stringify(this.loginForm.value));
		this.router.navigateByUrl('dashboard');
	}
}
