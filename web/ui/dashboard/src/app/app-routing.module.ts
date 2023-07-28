import { inject, NgModule } from '@angular/core';
import { RouterModule, Routes } from '@angular/router';
import { LoginService } from './public/login/login.service';

export const getSignupConfig = async (loginService = inject(LoginService)) => await loginService.getSignupConfig();

const routes: Routes = [
	{
		path: '',
		loadChildren: () => import('./private/private.module').then(m => m.PrivateModule)
	},
	{
		path: 'portal',
		loadChildren: () => import('./public/app/app.module').then(m => m.AppModule)
	},
	{
		path: 'portal/event-deliveries/:id',
		loadChildren: () => import('./public/event-delivery/event-delivery.module').then(m => m.EventDeliveryModule)
	},
	{
		path: 'portal/subscriptions/new',
		loadChildren: () => import('./public/create-subscription/create-subscription.module').then(m => m.CreateSubscriptionPublicModule)
	},
	{
		path: 'portal/subscriptions/:id',
		loadChildren: () => import('./public/app/app.module').then(m => m.AppModule)
	},
	{
		path: '',
		resolve: [() => getSignupConfig()],
		loadChildren: () => import('./public/public.module').then(m => m.PublicModule)
	}
];

@NgModule({
	imports: [RouterModule.forRoot(routes)],
	exports: [RouterModule]
})
export class AppRoutingModule {}
