import { inject, NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ProjectComponent } from './project.component';
import { Routes, RouterModule, Router, ActivatedRouteSnapshot, RouterStateSnapshot } from '@angular/router';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { GithubStarComponent } from 'src/app/components/github-star/github-star.component';
import { ListItemComponent } from 'src/app/components/list-item/list-item.component';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { TooltipComponent } from 'src/app/components/tooltip/tooltip.component';
import { SkeletonLoaderComponent } from 'src/app/components/skeleton-loader/skeleton-loader.component';
import { PrivateService } from '../../private.service';
import { ProjectService } from './project.service';
import { BadgeComponent } from 'src/app/components/badge/badge.component';
import { DropdownComponent, DropdownOptionDirective } from 'src/app/components/dropdown/dropdown.component';
import { DropdownContainerComponent } from 'src/app/components/dropdown-container/dropdown-container.component';
import { LoaderModule } from '../../components/loader/loader.module';

export const projectResolver = async (route: ActivatedRouteSnapshot, _state: RouterStateSnapshot, privateService = inject(PrivateService), projectService = inject(ProjectService), router = inject(Router)) => {
	try {
		const projectDetails = await privateService.getProjectDetails({ projectId: route.params.id });
		projectService.activeProjectDetails = projectDetails.data;
		return projectDetails;
	} catch (error) {
		return router.navigateByUrl('/projects');
	}
};

const routes: Routes = [
	{
		path: '',
		component: ProjectComponent,
		resolve: { projectDetails: projectResolver },
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
				path: 'events-log',
				loadComponent: () => import('./event-logs/event-logs.component').then(m => m.EventLogsComponent)
			},
			{
				path: 'meta-events',
				loadComponent: () => import('./meta-events/meta-events.component').then(m => m.MetaEventsComponent)
			}
		]
	}
];

@NgModule({
	declarations: [ProjectComponent],
	imports: [
		CommonModule,
		RouterModule.forChild(routes),
		ButtonComponent,
		GithubStarComponent,
		ListItemComponent,
		GithubStarComponent,
		TagComponent,
		TooltipComponent,
		SkeletonLoaderComponent,
		BadgeComponent,
		DropdownComponent,
		DropdownContainerComponent,
		DropdownOptionDirective,
		LoaderModule
	]
})
export class ProjectModule {}
