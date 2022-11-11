import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TableLoaderComponent } from './table-loader.component';
import { TableComponent, TableCellComponent, TableRowComponent, TableHeadCellComponent, TableHeadComponent } from 'src/app/components/table/table.component';
import { SkeletonLoaderComponent } from 'src/app/components/skeleton-loader/skeleton-loader.component';

@NgModule({
	declarations: [TableLoaderComponent],
	imports: [CommonModule, TableComponent, TableHeadCellComponent, TableCellComponent, TableRowComponent, TableHeadComponent, SkeletonLoaderComponent],
	exports: [TableLoaderComponent]
})
export class TableLoaderModule {}
