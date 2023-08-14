import { NgModule } from '@angular/core';
import { CommonModule, DatePipe } from '@angular/common';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { PrismModule } from 'src/app/private/components/prism/prism.module';
import { LoaderModule } from 'src/app/private/components/loader/loader.module';
import { TableLoaderModule } from 'src/app/private/components/table-loader/table-loader.module';
import { RouterModule } from '@angular/router';
import { EventDeliveriesComponent } from './event-deliveries.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { ListItemComponent } from 'src/app/components/list-item/list-item.component';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { DialogDirective } from 'src/app/components/dialog/dialog.directive';
import { TableComponent, TableCellComponent, TableRowComponent, TableHeadCellComponent, TableHeadComponent } from 'src/app/components/table/table.component';
import { StatusColorModule } from 'src/app/pipes/status-color/status-color.module';
import { DropdownComponent, DropdownOptionDirective } from 'src/app/components/dropdown/dropdown.component';
import { DatePickerComponent } from 'src/app/components/date-picker/date-picker.component';
import { TimePickerComponent } from 'src/app/components/time-picker/time-picker.component';
import { TooltipComponent } from 'src/app/components/tooltip/tooltip.component';
import { PaginationComponent } from 'src/app/private/components/pagination/pagination.component';

@NgModule({
	declarations: [EventDeliveriesComponent],
	imports: [
		CommonModule,
		ReactiveFormsModule,
		FormsModule,
		LoaderModule,
		TableLoaderModule,
		PrismModule,
		RouterModule,
		CardComponent,
		ButtonComponent,
		DropdownComponent,
		ListItemComponent,
		EmptyStateComponent,
		TagComponent,
		StatusColorModule,
		TableHeadComponent,
		TableHeadCellComponent,
		TableRowComponent,
		TableCellComponent,
		TableComponent,
		TimePickerComponent,
		DatePickerComponent,
		DropdownOptionDirective,
		TooltipComponent,
		PaginationComponent,
        DialogDirective
	],
	exports: [EventDeliveriesComponent],
	providers: [DatePipe]
})
export class EventDeliveriesModule {}
