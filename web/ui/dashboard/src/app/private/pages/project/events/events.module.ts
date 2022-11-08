import { NgModule } from '@angular/core';
import { CommonModule, DatePipe } from '@angular/common';
import { EventsComponent } from './events.component';
import { Routes, RouterModule } from '@angular/router';
import { ReactiveFormsModule } from '@angular/forms';
import { EventModule } from './event/event.module';
import { EventDeliveriesModule } from './event-deliveries/event-deliveries.module';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { ListItemComponent } from 'src/app/components/list-item/list-item.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { ChartComponent } from 'src/app/components/chart/chart.component';
import { SkeletonLoaderComponent } from 'src/app/components/skeleton-loader/skeleton-loader.component';
import { DropdownComponent } from 'src/app/components/dropdown/dropdown.component';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { SendEventComponent } from '../apps/app-details/send-event/send-event.component';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { LoaderModule } from 'src/app/private/components/loader/loader.module';
import { DatePickerComponent } from 'src/app/components/date-picker/date-picker.component';

const routes: Routes = [{ path: '', component: EventsComponent }];

@NgModule({
	declarations: [EventsComponent],
	imports: [
		CommonModule,
		ReactiveFormsModule,
		RouterModule.forChild(routes),
		EventModule,
		EventDeliveriesModule,
		DropdownComponent,
		ButtonComponent,
		ListItemComponent,
		CardComponent,
		ChartComponent,
		SkeletonLoaderComponent,
		EmptyStateComponent,
		SendEventComponent,
		ModalComponent,
		LoaderModule,
        DatePickerComponent
	],
	providers: [DatePipe]
})
export class EventsModule {}
