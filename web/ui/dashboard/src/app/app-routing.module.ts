import { NgModule } from '@angular/core';
import { RouterModule, Routes } from '@angular/router';

const routes: Routes = [
	{
		path: '',
		loadChildren: () => import('./private/private.module').then(m => m.PrivateModule)
	},
	{
		path: 'portal',
		loadComponent: () => import('./portal/portal.component').then(m => m.PortalComponent),
		children: [
			{
				path: '',
				loadComponent: () => import('./portal/event-deliveries/event-deliveries.component').then(m => m.EventDeliveriesComponent)
			},
			{
				path: 'endpoints',
				loadComponent: () => import('./portal/endpoints/endpoints.component').then(m => m.EndpointsComponent)
			},
			{
				path: 'endpoints/:id',
				loadComponent: () => import('./portal/endpoints/endpoints.component').then(m => m.EndpointsComponent)
			},
			{
				path: 'subscriptions',
				loadComponent: () => import('./portal/subscriptions/subscriptions.component').then(m => m.SubscriptionsComponent)
			},
			{
				path: 'subscriptions/:id',
				loadComponent: () => import('./portal/subscriptions/subscriptions.component').then(m => m.SubscriptionsComponent)
			},
			{
				path: 'event-deliveries/:id',
				loadComponent: () => import('./portal/event-delivery/event-delivery.component').then(m => m.EventDeliveryComponent)
			}
		]
	},
	{
		path: '',
		loadChildren: () => import('./public/public.module').then(m => m.PublicModule)
	},
    {
		path: 'portal/events',
		loadComponent: () => import('./public/events-catalogue/events-catalogue.component').then(m => m.EventsCatalogueComponent)
	},
];

@NgModule({
	imports: [RouterModule.forRoot(routes)],
	exports: [RouterModule]
})
export class AppRoutingModule {}
