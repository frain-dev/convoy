import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { PrismModule } from 'src/app/private/components/prism/prism.module';
import { LoaderModule } from 'src/app/private/components/loader/loader.module';
import { TableLoaderModule } from 'src/app/private/components/table-loader/table-loader.module';
import { EventComponent } from './event.component';
import { RouterModule } from '@angular/router';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { ListItemComponent } from 'src/app/components/list-item/list-item.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { TableComponent, TableCellComponent, TableRowComponent, TableHeadCellComponent, TableHeadComponent } from 'src/app/components/table/table.component';
import { StatusColorModule } from 'src/app/pipes/status-color/status-color.module';
import { DropdownComponent } from 'src/app/components/dropdown/dropdown.component';
import { DatePickerComponent } from 'src/app/components/date-picker/date-picker.component';
import { TimePickerComponent } from 'src/app/components/time-picker/time-picker.component';

@NgModule({
	declarations: [EventComponent],
	imports: [
		CommonModule,
		ReactiveFormsModule,
		FormsModule,
		LoaderModule,
		TableLoaderModule,
		PrismModule,
		RouterModule,

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
		TableComponent,
		TimePickerComponent,
		DatePickerComponent
	],
	exports: [EventComponent]
})
export class EventModule {}
