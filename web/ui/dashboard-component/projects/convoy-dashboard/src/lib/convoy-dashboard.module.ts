import { NgModule } from '@angular/core';
import { ConvoyDashboardComponent } from './convoy-dashboard.component';
import { MatDatepickerModule } from '@angular/material/datepicker';
import { MatNativeDateModule } from '@angular/material/core';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { CommonModule, DatePipe } from '@angular/common';
import { PrismModule } from './prism/prism.module';
import { ConvoyLoaderComponent } from './shared-components/loader.component';
import { SharedModule } from './shared/shared.module';
import { RouterModule } from '@angular/router';
import { ConvoyTableLoaderComponent } from './shared-components/table-loader.component';
import { ConvoyNotificationComponent } from './shared-components/notification.component';
import { TimeFilterComponent } from './shared-components/time-filter.component';
import { AppDetailsComponent } from './shared-components/app-details/app-details.component';
import { EventDeliveryDetailsComponent } from './shared-components/event-delivery-details/event-delivery-details.component';

@NgModule({
	declarations: [ConvoyDashboardComponent, ConvoyLoaderComponent, ConvoyTableLoaderComponent, ConvoyNotificationComponent, TimeFilterComponent, EventDeliveryDetailsComponent, AppDetailsComponent],
	imports: [CommonModule, MatDatepickerModule, MatNativeDateModule, FormsModule, ReactiveFormsModule, PrismModule, SharedModule, RouterModule],
	exports: [ConvoyDashboardComponent],
	providers: [DatePipe]
})
export class ConvoyDashboardModule {}
