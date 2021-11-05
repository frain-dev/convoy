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
	}
];

@NgModule({
	imports: [RouterModule.forRoot(routes)],
	exports: [RouterModule]
})
export class AppRoutingModule {}
