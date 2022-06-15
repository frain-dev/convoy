import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { PrismModule } from 'src/app/private/components/prism/prism.module';
import { RouterModule } from '@angular/router';
import { EventDeliveryDetailsComponent } from './event-delivery-details.component';
import { LoaderModule } from 'src/app/private/components/loader/loader.module';

@NgModule({
	declarations: [EventDeliveryDetailsComponent],
	imports: [CommonModule, RouterModule, PrismModule, LoaderModule],
	exports: [EventDeliveryDetailsComponent]
})
export class EventDeliveryDetailsModule {}
