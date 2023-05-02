import { inject } from '@angular/core';
import { Routes } from '@angular/router';
import { PrivateComponent } from './private.component';
import { PrivateService } from './private.service';

export const fetchOrganisations = async (privateService = inject(PrivateService)) => await privateService.getOrganizations();
export const fetchMembership = async (privateService = inject(PrivateService)) => await privateService.getOrganizationMembership();

const routes: Routes = [
	{
		path: '',
		component: PrivateComponent,
		resolve: [() => fetchOrganisations(), () => fetchMembership()],
		children: [
			{
				path: '',
				redirectTo: 'projects',
				pathMatch: 'full'
			},
			{
				path: 'projects',
				loadChildren: () => import('./pages/projects/projects.module').then(m => m.ProjectsModule)
			},
			{
				path: 'projects/new',
				loadChildren: () => import('./pages/create-project/create-project.module').then(m => m.CreateProjectModule)
			},
			{
				path: 'projects/:id/setup',
				loadComponent: () => import('./pages/setup-project/setup-project.component').then(mod => mod.SetupProjectComponent)
			},
			{
				path: 'projects/:id',
				loadChildren: () => import('./pages/project/project.module').then(m => m.ProjectModule)
			},
			{
				path: 'app-portal/:token',
				loadChildren: () => import('./pages/app/app.module').then(m => m.AppModule)
			},
			{
				path: 'team',
				loadChildren: () => import('./pages/teams/teams.module').then(m => m.TeamsModule)
			},
			{
				path: 'user-settings',
				loadChildren: () => import('./pages/account/account.module').then(m => m.AccountModule)
			},
			{
				path: 'settings',
				loadChildren: () => import('./pages/settings/settings.module').then(m => m.SettingsModule)
			},
			{
				path: 'get-started',
				loadComponent: () => import('./pages/onboarding/onboarding.component').then(mod => mod.OnboardingComponent)
			}
		]
	}
];

export { routes };
