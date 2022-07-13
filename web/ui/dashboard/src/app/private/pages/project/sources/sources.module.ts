import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { SourcesComponent } from './sources.component';
import { Routes, RouterModule } from '@angular/router';
import { CreateSourceModule } from 'src/app/private/components/create-source/create-source.module';
import { TableLoaderModule } from 'src/app/private/components/table-loader/table-loader.module';
import { TableCellComponent } from 'src/app/components/table-cell/table-cell.component';
import { TableHeadCellComponent } from 'src/app/components/table-head-cell/table-head-cell.component';
import { TableHeadComponent } from 'src/app/components/table-head/table-head.component';
import { TableRowComponent } from 'src/app/components/table-row/table-row.component';
import { TableComponent } from 'src/app/components/table/table.component';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { ListItemComponent } from 'src/app/components/list-item/list-item.component';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { DeleteModalModule } from 'src/app/private/components/delete-modal/delete-modal.module';

const routes: Routes = [{ path: '', component: SourcesComponent }];

@NgModule({
	declarations: [SourcesComponent],
	imports: [CommonModule, TableLoaderModule, RouterModule.forChild(routes), CreateSourceModule, DeleteModalModule, TableHeadComponent, TableHeadCellComponent, TableRowComponent, TableCellComponent, TableComponent, TagComponent, ButtonComponent, ListItemComponent, EmptyStateComponent, CardComponent]
})
export class SourcesModule {}
