import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { AppsComponent } from './apps.component';
import { RouterModule, Routes } from '@angular/router';
import { CreateAppModule } from 'src/app/private/components/create-app/create-app.module';
import { FormsModule } from '@angular/forms';
import { TableLoaderModule } from 'src/app/private/components/table-loader/table-loader.module';
import { DeleteModalModule } from 'src/app/private/components/delete-modal/delete-modal.module';

const routes: Routes = [
	{
		path: '',
		component: AppsComponent
	},
	{
		path: 'new',
		component: AppsComponent
	},
	{
		path: ':id/edit',
		component: AppsComponent
	},
	{
		path: ':id',
		loadChildren: () => import('./app-details/app-details.module').then(m => m.AppDetailsModule)
	}
];

@NgModule({
	declarations: [AppsComponent],
	imports: [CommonModule, CreateAppModule, FormsModule, TableLoaderModule, DeleteModalModule, RouterModule.forChild(routes)]
})
export class AppsModule {}
