import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { EventDeliveryComponent } from './event-delivery.component';
import { Routes, RouterModule } from '@angular/router';
import { EventDeliveryDetailsModule } from 'src/app/private/pages/project/events/event-delivery-details/event-delivery-details.module';
import { PageDirective } from 'src/app/components/page/page.component';

const routes: Routes = [{ path: '', component: EventDeliveryComponent }];

@NgModule({
	declarations: [EventDeliveryComponent],
	imports: [CommonModule, RouterModule.forChild(routes), EventDeliveryDetailsModule, PageDirective]
})
export class EventDeliveryModule {}
