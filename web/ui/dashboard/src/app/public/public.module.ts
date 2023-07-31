import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { PublicComponent } from './public.component';
import { RouterModule, Routes } from '@angular/router';

const routes: Routes = [
	{
		path: '',
		component: PublicComponent,
		children: [
			{
				path: 'login',
				loadComponent: () => import('./login/login.component').then(mod => mod.LoginComponent)
			},
			{
				path: 'signup',
				loadComponent: () => import('./signup/signup.component').then(mod => mod.SignupComponent)
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
				path: 'verify-email',
				loadComponent: () => import('./verify-email/verify-email.component').then(mod => mod.VerifyEmailComponent)
			}
		]
	}
];

@NgModule({
	declarations: [PublicComponent],
	imports: [CommonModule, RouterModule.forChild(routes)]
})
export class PublicModule {}
