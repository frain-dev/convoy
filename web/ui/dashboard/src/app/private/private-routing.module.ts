import { NgModule } from '@angular/core';
import { RouterModule, Routes } from '@angular/router';
import { PrivateComponent } from './private.component';

const routes: Routes = [
	{
		path: '',
		component: PrivateComponent,
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
				path: 'projects/:id',
				loadChildren: () => import('./pages/project/project.module').then(m => m.ProjectModule)
			},
			{
				path: 'app-portal/:token',
				loadChildren: () => import('./pages/app/app.module').then(m => m.AppModule)
			},
			{
				path: 'organisation-settings',
				loadChildren: () => import('./pages/organisation/organisation.module').then(m => m.OrganisationModule)
			},
			{
				path: 'account',
				loadChildren: () => import('./pages/account/account.module').then(m => m.AccountModule)
			},
		]
	}
];

@NgModule({
	imports: [RouterModule.forChild(routes)],
	exports: [RouterModule]
})
export class PrivateRoutingModule {}
