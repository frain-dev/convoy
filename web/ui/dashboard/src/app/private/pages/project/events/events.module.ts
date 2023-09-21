import { NgModule } from '@angular/core';
import { CommonModule, DatePipe } from '@angular/common';
import { EventsComponent } from './events.component';
import { Routes, RouterModule } from '@angular/router';
import { ReactiveFormsModule } from '@angular/forms';
import { EventDeliveriesModule } from './event-deliveries/event-deliveries.module';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { ListItemComponent } from 'src/app/components/list-item/list-item.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { ChartComponent } from 'src/app/components/chart/chart.component';
import { SkeletonLoaderComponent } from 'src/app/components/skeleton-loader/skeleton-loader.component';
import { DropdownComponent, DropdownOptionDirective } from 'src/app/components/dropdown/dropdown.component';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { LoaderModule } from 'src/app/private/components/loader/loader.module';
import { DatePickerComponent } from 'src/app/components/date-picker/date-picker.component';
import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { StatusColorModule } from 'src/app/pipes/status-color/status-color.module';

const routes: Routes = [{ path: '', component: EventsComponent }];

@NgModule({
	declarations: [EventsComponent],
	imports: [
		CommonModule,
		ReactiveFormsModule,
		RouterModule.forChild(routes),
		EventDeliveriesModule,
		DropdownComponent,
		ButtonComponent,
		ListItemComponent,
		CardComponent,
		ChartComponent,
		SkeletonLoaderComponent,
		EmptyStateComponent,
		LoaderModule,
		DatePickerComponent,
		DropdownOptionDirective,
		CopyButtonComponent,
		TagComponent,
		StatusColorModule
	],
	providers: [DatePipe]
})
export class EventsModule {}
