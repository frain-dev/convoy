import { NgModule } from '@angular/core';
import { CommonModule, DatePipe } from '@angular/common';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { TimeFilterModule } from 'src/app/private/components/time-filter/time-filter.module';
import { PrismModule } from 'src/app/private/components/prism/prism.module';
import { DateFilterModule } from 'src/app/private/components/date-filter/date-filter.module';
import { LoaderModule } from 'src/app/private/components/loader/loader.module';
import { TableLoaderModule } from 'src/app/private/components/table-loader/table-loader.module';
import { RouterModule } from '@angular/router';
import { EventDeliveriesComponent } from './event-deliveries.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { DropdownComponent } from 'src/app/components/dropdown/dropdown.component';
import { ListItemComponent } from 'src/app/components/list-item/list-item.component';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { PipesModule } from 'src/app/pipes/pipes.module';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { TableCellComponent } from 'src/app/components/table-cell/table-cell.component';
import { TableHeadCellComponent } from 'src/app/components/table-head-cell/table-head-cell.component';
import { TableHeadComponent } from 'src/app/components/table-head/table-head.component';
import { TableRowComponent } from 'src/app/components/table-row/table-row.component';
import { TableComponent } from 'src/app/components/table/table.component';

@NgModule({
	declarations: [EventDeliveriesComponent],
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
		CardComponent,
		ButtonComponent,
		DropdownComponent,
		ListItemComponent,
		EmptyStateComponent,
		TagComponent,
		PipesModule,
		ModalComponent,
		TableHeadComponent,
		TableHeadCellComponent,
		TableRowComponent,
		TableCellComponent,
		TableComponent
	],
	exports: [EventDeliveriesComponent],
	providers: [DatePipe]
})
export class EventDeliveriesModule {}
