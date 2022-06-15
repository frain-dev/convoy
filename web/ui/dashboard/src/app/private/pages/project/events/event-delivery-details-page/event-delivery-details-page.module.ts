import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { EventDeliveryDetailsPageComponent } from './event-delivery-details-page.component';
import { RouterModule, Routes } from '@angular/router';
import { EventDeliveryDetailsModule } from '../event-delivery-details/event-delivery-details.module';

const routes: Routes = [{ path: '', component: EventDeliveryDetailsPageComponent }];

@NgModule({
	declarations: [EventDeliveryDetailsPageComponent],
	imports: [CommonModule, RouterModule.forChild(routes), EventDeliveryDetailsModule]
})
export class EventDeliveryDetailsPageModule {}
