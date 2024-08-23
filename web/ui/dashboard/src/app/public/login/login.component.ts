import { CommonModule } from '@angular/common';
import { Component, OnInit } from '@angular/core';
import { FormBuilder, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent, PasswordInputFieldComponent } from 'src/app/components/input/input.component';
import { LoginService } from './login.service';
import { LoaderModule } from 'src/app/private/components/loader/loader.module';
import { PrivateService } from 'src/app/private/private.service';
import { ORGANIZATION_DATA } from 'src/app/models/organisation.model';
import { SignupService } from '../signup/signup.service';
import { LicensesService } from 'src/app/services/licenses/licenses.service';

@Component({
	selector: 'app-login',
	standalone: true,
	imports: [CommonModule, ReactiveFormsModule, ButtonComponent, InputFieldDirective, InputDirective, LabelComponent, InputErrorComponent, PasswordInputFieldComponent, LoaderModule],
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
	isLoadingProject = false;
	isFetchingConfig = false;
	isSignupEnabled = false;
	organisations?: ORGANIZATION_DATA[];

	constructor(private formBuilder: FormBuilder, public router: Router, private loginService: LoginService, private signupService: SignupService, private privateService: PrivateService, private licenseService: LicensesService) {}

	ngOnInit() {
		this.getSignUpConfig();
		this.licenseService.setLicenses();
	}

	async getSignUpConfig() {
		this.isFetchingConfig = true;
		try {
			const response = await this.signupService.getSignupConfig();
			this.isSignupEnabled = response.data;
			this.isFetchingConfig = false;
		} catch (error) {
			this.isFetchingConfig = false;
		}
	}

	async login() {
		if (this.loginForm.invalid) return this.loginForm.markAllAsTouched();

		this.disableLoginBtn = true;
		try {
			const response: any = await this.loginService.login(this.loginForm.value);
			localStorage.setItem('CONVOY_AUTH', JSON.stringify(response.data));
			localStorage.setItem('CONVOY_AUTH_TOKENS', JSON.stringify(response.data.token));

			this.isLoadingProject = true;
			await this.getOrganisations();
			return this.router.navigateByUrl('/');
		} catch {
			return (this.disableLoginBtn = false);
		}
	}

	async getOrganisations() {
		try {
			await this.privateService.getOrganizations({ refresh: true });
			return;
		} catch (error) {
			return error;
		}
	}
}
