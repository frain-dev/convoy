import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TeamsComponent } from './teams.component';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { RouterModule, Routes } from '@angular/router';
import { TableLoaderModule } from '../../components/table-loader/table-loader.module';
import { DeleteModalModule } from '../../components/delete-modal/delete-modal.module';
import { PageComponent } from 'src/app/components/page/page.component';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { BadgeComponent } from 'src/app/components/badge/badge.component';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { InputComponent } from 'src/app/components/input/input.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { TableComponent } from 'src/app/components/table/table.component';
import { TableHeadComponent } from 'src/app/components/table-head/table-head.component';
import { TableHeadCellComponent } from 'src/app/components/table-head-cell/table-head-cell.component';
import { TableCellComponent } from 'src/app/components/table-cell/table-cell.component';
import { TableRowComponent } from 'src/app/components/table-row/table-row.component';
import { ListItemComponent } from 'src/app/components/list-item/list-item.component';
import { DropdownComponent } from 'src/app/components/dropdown/dropdown.component';

const routes: Routes = [
	{ path: '', component: TeamsComponent },
	{ path: 'new', component: TeamsComponent }
];

@NgModule({
	declarations: [TeamsComponent],
	imports: [
		CommonModule,
		FormsModule,
		TableLoaderModule,
		ReactiveFormsModule,
		DeleteModalModule,
		PageComponent,
		ModalComponent,
		DropdownComponent,
		ButtonComponent,
		BadgeComponent,
		EmptyStateComponent,
		InputComponent,
		CardComponent,
		TableComponent,
		TableHeadComponent,
		TableHeadCellComponent,
		TableCellComponent,
		TableRowComponent,
		RouterModule.forChild(routes),
		ListItemComponent
	]
})
export class TeamsModule {}
