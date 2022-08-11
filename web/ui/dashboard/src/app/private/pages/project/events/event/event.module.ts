import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { TimeFilterModule } from 'src/app/private/components/time-filter/time-filter.module';
import { PrismModule } from 'src/app/private/components/prism/prism.module';
import { DateFilterModule } from 'src/app/private/components/date-filter/date-filter.module';
import { LoaderModule } from 'src/app/private/components/loader/loader.module';
import { TableLoaderModule } from 'src/app/private/components/table-loader/table-loader.module';
import { EventComponent } from './event.component';
import { RouterModule } from '@angular/router';
import { InputComponent } from 'src/app/components/input/input.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { DropdownComponent } from 'src/app/components/dropdown/dropdown.component';
import { EmptyStateComponent } from 'src/stories/empty-state/empty-state.component';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { ListItemComponent } from 'src/app/components/list-item/list-item.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { TableHeadComponent } from 'src/app/components/table-head/table-head.component';
import { TableHeadCellComponent } from 'src/app/components/table-head-cell/table-head-cell.component';
import { TableRowComponent } from 'src/app/components/table-row/table-row.component';
import { TableCellComponent } from 'src/app/components/table-cell/table-cell.component';
import { TableComponent } from 'src/app/components/table/table.component';
import { StatusColorModule } from 'src/app/pipes/status-color/status-color.module';

@NgModule({
	declarations: [EventComponent],
	imports: [
		CommonModule,
		ReactiveFormsModule,
		FormsModule,
		DateFilterModule,
		TimeFilterModule,
		LoaderModule,
		TableLoaderModule,
		PrismModule,
		RouterModule,
		InputComponent,
		ButtonComponent,
		DropdownComponent,
		EmptyStateComponent,
		TagComponent,
		ListItemComponent,
		CardComponent,
		StatusColorModule,
		TableHeadComponent,
		TableHeadCellComponent,
		TableRowComponent,
		TableCellComponent,
		TableComponent
	],
	exports: [EventComponent]
})
export class EventModule {}
