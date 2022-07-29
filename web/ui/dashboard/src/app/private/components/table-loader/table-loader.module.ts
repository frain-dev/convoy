import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TableLoaderComponent } from './table-loader.component';
import { TableComponent } from 'src/app/components/table/table.component';
import { TableHeadCellComponent } from 'src/app/components/table-head-cell/table-head-cell.component';
import { TableCellComponent } from 'src/app/components/table-cell/table-cell.component';
import { TableRowComponent } from 'src/app/components/table-row/table-row.component';
import { TableHeadComponent } from 'src/app/components/table-head/table-head.component';
import { SkeletonLoaderComponent } from 'src/app/components/skeleton-loader/skeleton-loader.component';

@NgModule({
	declarations: [TableLoaderComponent],
	imports: [CommonModule, TableComponent, TableHeadCellComponent, TableCellComponent, TableRowComponent, TableHeadComponent, SkeletonLoaderComponent],
	exports: [TableLoaderComponent]
})
export class TableLoaderModule {}
