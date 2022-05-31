import { NgModule } from '@angular/core';
import { CommonModule, DatePipe } from '@angular/common';
import { EventsComponent } from './events.component';
import { Routes, RouterModule } from '@angular/router';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { MatDatepickerModule } from '@angular/material/datepicker';
import { MatNativeDateModule } from '@angular/material/core';
import { TimeFilterModule } from 'src/app/private/components/time-filter/time-filter.module';
import { EventComponent } from './event/event.component';
import { EventDeliveriesComponent } from './event-deliveries/event-deliveries.component';
import { PrismModule } from 'src/app/private/components/prism/prism.module';
import { EventDeliveryDetailsComponent } from './event-delivery-details/event-delivery-details.component';
import { DateFilterModule } from 'src/app/private/components/date-filter/date-filter.module';
import { LoaderModule } from 'src/app/private/components/loader/loader.module';

const routes: Routes = [
	{ path: '', component: EventsComponent },
	{ path: ':eventId/delivery/:id', component: EventDeliveryDetailsComponent }
];

@NgModule({
	declarations: [EventsComponent, EventComponent, EventDeliveriesComponent, EventDeliveryDetailsComponent],
	imports: [CommonModule, ReactiveFormsModule, FormsModule, MatDatepickerModule, MatNativeDateModule, DateFilterModule, TimeFilterModule, LoaderModule, PrismModule, RouterModule.forChild(routes)],
	providers: [DatePipe]
})
export class EventsModule {}
