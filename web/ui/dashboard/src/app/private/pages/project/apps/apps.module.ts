import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { AppsComponent } from './apps.component';
import { RouterModule, Routes } from '@angular/router';
import { CreateAppModule } from 'src/app/private/components/create-app/create-app.module';
import { FormsModule } from '@angular/forms';
import { TableLoaderModule } from 'src/app/private/components/table-loader/table-loader.module';
import { TableComponent, TableCellComponent, TableRowComponent, TableHeadCellComponent, TableHeadComponent } from 'src/app/components/table/table.component';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { ListItemComponent } from 'src/app/components/list-item/list-item.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { DropdownComponent } from 'src/app/components/dropdown/dropdown.component';
import { DeleteModalComponent } from 'src/app/private/components/delete-modal/delete-modal.component';

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
	imports: [
		CommonModule,
		CreateAppModule,
		FormsModule,
		TableLoaderModule,
		DeleteModalComponent,
		RouterModule.forChild(routes),
		TableHeadComponent,
		TableHeadCellComponent,
		TableRowComponent,
		TableCellComponent,
		TableComponent,
		TagComponent,
		ButtonComponent,
		DropdownComponent,
		ListItemComponent,
		CardComponent,
		EmptyStateComponent,
		ModalComponent
	]
})
export class AppsModule {}
