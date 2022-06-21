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

@NgModule({
	declarations: [EventDeliveriesComponent],
	imports: [CommonModule, ReactiveFormsModule, FormsModule, MatDatepickerModule, MatNativeDateModule, DateFilterModule, TimeFilterModule, LoaderModule, TableLoaderModule, PrismModule, RouterModule],
	exports: [EventDeliveriesComponent],
	providers: [DatePipe]
})
export class EventDeliveriesModule {}
