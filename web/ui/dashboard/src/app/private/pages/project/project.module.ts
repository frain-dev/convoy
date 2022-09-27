import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ProjectComponent } from './project.component';
import { Routes, RouterModule } from '@angular/router';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { GithubStarComponent } from 'src/app/components/github-star/github-star.component';
import { ListItemComponent } from 'src/app/components/list-item/list-item.component';

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
				path: 'apps',
				loadChildren: () => import('./apps/apps.module').then(m => m.AppsModule)
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
			}
		]
	}
];

@NgModule({
	declarations: [ProjectComponent],
	imports: [CommonModule, RouterModule.forChild(routes), ButtonComponent, GithubStarComponent, ListItemComponent]
})
export class ProjectModule {}
