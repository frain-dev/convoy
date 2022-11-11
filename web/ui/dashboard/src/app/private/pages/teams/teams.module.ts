import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TeamsComponent } from './teams.component';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { RouterModule, Routes } from '@angular/router';
import { TableLoaderModule } from '../../components/table-loader/table-loader.module';
import { PageComponent } from 'src/app/components/page/page.component';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent } from 'src/app/components/input/input.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { TableComponent, TableCellComponent, TableRowComponent, TableHeadCellComponent, TableHeadComponent } from 'src/app/components/table/table.component';
import { ListItemComponent } from 'src/app/components/list-item/list-item.component';
import { BadgeComponent } from 'src/app/components/badge/badge.component';
import { DropdownComponent } from 'src/app/components/dropdown/dropdown.component';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { DeleteModalComponent } from '../../components/delete-modal/delete-modal.component';

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
		DeleteModalComponent,
		PageComponent,
		ModalComponent,
		DropdownComponent,
		ButtonComponent,
		BadgeComponent,
		EmptyStateComponent,

		CardComponent,
		TableComponent,
		TableHeadComponent,
		TableHeadCellComponent,
		TableCellComponent,
		TableRowComponent,
		RouterModule.forChild(routes),
		ListItemComponent,
		InputFieldDirective,
		InputErrorComponent,
		InputDirective,
		LabelComponent
	]
})
export class TeamsModule {}
