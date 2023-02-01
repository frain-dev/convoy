import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { SourcesComponent } from './sources.component';
import { Routes, RouterModule } from '@angular/router';
import { CreateSourceModule } from 'src/app/private/components/create-source/create-source.module';
import { TableLoaderModule } from 'src/app/private/components/table-loader/table-loader.module';
import { TableComponent, TableCellComponent, TableRowComponent, TableHeadCellComponent, TableHeadComponent } from 'src/app/components/table/table.component';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { ListItemComponent } from 'src/app/components/list-item/list-item.component';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';
import { SourceValueModule } from 'src/app/pipes/source-value/source-value.module';
import { DeleteModalComponent } from 'src/app/private/components/delete-modal/delete-modal.component';
import { DropdownComponent, DropdownOptionDirective } from 'src/app/components/dropdown/dropdown.component';

const routes: Routes = [{ path: '', component: SourcesComponent }];

@NgModule({
	declarations: [SourcesComponent],
	imports: [
		CommonModule,
		TableLoaderModule,
		RouterModule.forChild(routes),
		CreateSourceModule,
		DeleteModalComponent,
		TableHeadComponent,
		TableHeadCellComponent,
		TableRowComponent,
		TableCellComponent,
		TableComponent,
		TagComponent,
		ButtonComponent,
		ListItemComponent,
		EmptyStateComponent,
		CardComponent,
		ModalComponent,
		CopyButtonComponent,
		SourceValueModule,
		CopyButtonComponent,
        DropdownComponent,
        DropdownOptionDirective
	]
})
export class SourcesModule {}
