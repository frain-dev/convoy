import { NgModule } from '@angular/core';
import { RouterModule, Routes } from '@angular/router';

const routes: Routes = [
	{
		path: '',
		loadChildren: () => import('./private/private.module').then(m => m.PrivateModule)
	},
	{
		path: 'login',
		loadChildren: () => import('./public/login/login.module').then(m => m.LoginModule)
	},
	{
		path: 'app/:token',
		loadChildren: () => import('./public/app/app.module').then(m => m.AppModule)
	},
	{
		path: 'app/:token/event-deliveries/:id',
		loadChildren: () => import('./public/event-delivery/event-delivery.module').then(m => m.EventDeliveryModule)
	},
	{
		path: 'app/:token/subscriptions/new',
		loadChildren: () => import('./public/create-subscription/create-subscription.module').then(m => m.CreateSubscriptionPublicModule)
	},
	{
		path: 'app/:token/subscriptions/:id',
		loadChildren: () => import('./public/app/app.module').then(m => m.AppModule)
  },
  {
		path: 'forgot-password',
		loadChildren: () => import('./public/forgot-password/forgot-password.module').then(m => m.ForgotPasswordModule)
	},
	{
		path: 'reset-password',
		loadChildren: () => import('./public/reset-password/reset-password.module').then(m => m.ResetPasswordModule)
	},
	{
		path: 'accept-invite',
		loadChildren: () => import('./public/accept-invite/accept-invite.module').then(m => m.AcceptInviteModule)
	}
];

@NgModule({
	imports: [RouterModule.forRoot(routes)],
	exports: [RouterModule]
})
export class AppRoutingModule {}
