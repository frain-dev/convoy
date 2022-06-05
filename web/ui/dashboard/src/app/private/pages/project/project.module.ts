import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ProjectComponent } from './project.component';
import { Routes, RouterModule } from '@angular/router';

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
				path: 'sources',
				loadChildren: () => import('./sources/sources.module').then(m => m.SourcesModule)
			},
			{
				path: 'sources/new',
				loadChildren: () => import('./sources/sources.module').then(m => m.SourcesModule)
			},
			{
				path: 'apps',
				loadChildren: () => import('./apps/apps.module').then(m => m.AppsModule)
			},
			{
				path: 'subscriptions',
				loadChildren: () => import('./subscriptions/subscriptions.module').then(m => m.SubscriptionsModule)
			},
			{
				path: 'subscriptions/new',
				loadChildren: () => import('./subscriptions/subscriptions.module').then(m => m.SubscriptionsModule)
			}
		]
	}
];

@NgModule({
	declarations: [ProjectComponent],
	imports: [CommonModule, RouterModule.forChild(routes)]
})
export class ProjectModule {}
