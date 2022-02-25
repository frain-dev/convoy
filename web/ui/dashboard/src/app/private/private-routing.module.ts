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
				redirectTo: 'dashboard',
				pathMatch: 'full'
			},
			{
				path: 'dashboard',
				loadChildren: () => import('./pages/dashboard/dashboard.module').then(m => m.DashboardModule)
			},
			{
				path: '/ui/app-portal',
				loadChildren: () => import('./pages/app/app.module').then(m => m.AppModule)
			},
			{
				path: '/ui/app-portal/:token',
				loadChildren: () => import('./pages/app/app.module').then(m => m.AppModule)
			}
		]
	}
];

@NgModule({
	imports: [RouterModule.forChild(routes)],
	exports: [RouterModule]
})
export class PrivateRoutingModule {}
