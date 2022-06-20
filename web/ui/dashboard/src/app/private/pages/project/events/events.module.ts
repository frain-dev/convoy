import { NgModule } from '@angular/core';
import { CommonModule, DatePipe } from '@angular/common';
import { EventsComponent } from './events.component';
import { Routes, RouterModule } from '@angular/router';
import { ReactiveFormsModule } from '@angular/forms';
import { DateFilterModule } from 'src/app/private/components/date-filter/date-filter.module';
import { LoaderModule } from 'src/app/private/components/loader/loader.module';
import { EventModule } from './event/event.module';
import { EventDeliveriesModule } from './event-deliveries/event-deliveries.module';

const routes: Routes = [{ path: '', component: EventsComponent }];

@NgModule({
	declarations: [EventsComponent],
	imports: [CommonModule, ReactiveFormsModule, DateFilterModule, LoaderModule, RouterModule.forChild(routes), EventModule, EventDeliveriesModule],
	providers: [DatePipe]
})
export class EventsModule {}
