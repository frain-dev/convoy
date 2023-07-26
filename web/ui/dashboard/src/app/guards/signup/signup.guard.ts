import { Injectable } from '@angular/core';
import { CanActivate, Router } from '@angular/router';
import { LoginService } from 'src/app/public/login/login.service';

@Injectable({
	providedIn: 'root'
})
export class SignupGuard implements CanActivate {
	constructor(private router: Router, private loginService: LoginService) {}

	canActivate() {
		const isSignupEnabled = this.loginService.signupConfig && location.hostname !== 'dashboard.getconvoy.io';

		if (!isSignupEnabled) {
			this.router.navigate(['login']);
			return false;
		}

		return true;
	}
}
