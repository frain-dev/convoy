import { NgModule } from '@angular/core';
import { CommonModule, DatePipe } from '@angular/common';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { MatDatepickerModule } from '@angular/material/datepicker';
import { MatNativeDateModule } from '@angular/material/core';
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

@NgModule({
	declarations: [EventDeliveriesComponent],
	imports: [
		CommonModule,
		ReactiveFormsModule,
		FormsModule,
		MatDatepickerModule,
		MatNativeDateModule,
		DateFilterModule,
		TimeFilterModule,
		LoaderModule,
		TableLoaderModule,
		PrismModule,
		RouterModule,
		CardComponent,
		ButtonComponent,
		DropdownComponent
	],
	exports: [EventDeliveriesComponent],
	providers: [DatePipe]
})
export class EventDeliveriesModule {}
