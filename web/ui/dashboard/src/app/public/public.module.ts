import {NgModule} from '@angular/core';
import {CommonModule} from '@angular/common';
import {PublicComponent} from './public.component';
import {RouterModule, Routes} from '@angular/router';
import {GoogleOAuthSetupGuard} from '../guards/google-oauth-setup.guard';

const routes: Routes = [
	// Redesigned pages render their own full-screen auth shell, outside the
	// legacy PublicComponent chrome. Pages migrate here as they are redesigned.
	{
		path: 'signup',
		loadComponent: () => import('./signup/signup.component').then(mod => mod.SignupComponent)
	},
	{
		path: 'login',
		loadComponent: () => import('./login/login.component').then(mod => mod.LoginComponent)
	},
	{
		path: 'email-verification',
		loadComponent: () => import('./email-verification/email-verification.component').then(mod => mod.EmailVerificationComponent)
	},
	{
		path: 'forgot-password',
		loadComponent: () => import('./forgot-password/forgot-password.component').then(mod => mod.ForgotPasswordComponent)
	},
	{
		path: 'reset-password',
		loadComponent: () => import('./reset-password/reset-password.component').then(mod => mod.ResetPasswordComponent)
	},
	{
		path: 'accept-invite',
		loadComponent: () => import('./accept-invite/accept-invite.component').then(mod => mod.AcceptInviteComponent)
	},
	{
		path: '',
		component: PublicComponent,
		children: [
			{
				path: 'verify-email',
				loadComponent: () => import('./verify-email/verify-email.component').then(mod => mod.VerifyEmailComponent)
			},
			{
				path: 'saml',
				loadComponent: () => import('./saml/saml.component').then(mod => mod.SamlComponent)
			},

			{
				path: 'google-oauth-setup',
				loadComponent: () => import('./google-oauth-setup/google-oauth-setup.component').then(mod => mod.GoogleOAuthSetupComponent),
				canActivate: [GoogleOAuthSetupGuard]
			},
		]
	}
];

@NgModule({
	declarations: [PublicComponent],
	imports: [CommonModule, RouterModule.forChild(routes)]
})
export class PublicModule {}
