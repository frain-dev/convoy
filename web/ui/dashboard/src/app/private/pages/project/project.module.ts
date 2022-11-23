import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ProjectComponent } from './project.component';
import { Routes, RouterModule } from '@angular/router';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { GithubStarComponent } from 'src/app/components/github-star/github-star.component';
import { ListItemComponent } from 'src/app/components/list-item/list-item.component';
import { PortalLinksComponent } from './portal-links/portal-links.component';

const routes: Routes = [
	{
		path: '',
		component: ProjectComponent,
		children: [
			{
				path: '',
				redirectTo: 'events',
				pathMatch: 'full'
			},
			{
				path: 'events',
				loadChildren: () => import('./events/events.module').then(m => m.EventsModule)
			},
			{
				path: 'events/event-deliveries/:id',
				loadChildren: () => import('./events/event-delivery-details-page/event-delivery-details-page.module').then(m => m.EventDeliveryDetailsPageModule)
			},
			{
				path: 'sources',
				loadChildren: () => import('./sources/sources.module').then(m => m.SourcesModule)
			},
			{
				path: 'sources/:id',
				loadChildren: () => import('./sources/sources.module').then(m => m.SourcesModule)
			},
			{
				path: 'settings',
				loadChildren: () => import('./settings/settings.module').then(m => m.SettingsModule)
			},
			{
				path: 'subscriptions',
				loadChildren: () => import('./subscriptions/subscriptions.module').then(m => m.SubscriptionsModule)
			},
			{
				path: 'subscriptions/:id',
				loadChildren: () => import('./subscriptions/subscriptions.module').then(m => m.SubscriptionsModule)
			},
			{
				path: 'endpoints',
				loadComponent: () => import('./endpoints/endpoints.component').then(m => m.EndpointsComponent)
			},
			{
				path: 'endpoints/new',
				loadComponent: () => import('./endpoints/endpoints.component').then(m => m.EndpointsComponent)
			},
            {
				path: 'endpoints/:id',
				loadComponent: () => import('./endpoint-details/endpoint-details.component').then(m => m.EndpointDetailsComponent)
			},
            {
				path: 'endpoints/:id/edit',
				loadComponent: () => import('./endpoints/endpoints.component').then(m => m.EndpointsComponent)
			},
			{
				path: 'portal-links',
				loadComponent: () => import('./portal-links/portal-links.component').then(m => m.PortalLinksComponent)
			},
			{
				path: 'portal-links/new',
				loadComponent: () => import('./portal-links/portal-links.component').then(m => m.PortalLinksComponent)
			},
            {
				path: 'portal-links/:id/edit',
				loadComponent: () => import('./portal-links/portal-links.component').then(m => m.PortalLinksComponent)
			},
            {
				path: 'event-logs',
				loadComponent: () => import('./event-logs/event-logs.component').then(m => m.EventLogsComponent)
			}
		]
	}
];

@NgModule({
	declarations: [ProjectComponent],
	imports: [CommonModule, RouterModule.forChild(routes), ButtonComponent, GithubStarComponent, ListItemComponent, GithubStarComponent]
})
export class ProjectModule {}
