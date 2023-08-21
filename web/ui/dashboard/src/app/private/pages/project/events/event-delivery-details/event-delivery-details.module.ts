import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { PrismModule } from 'src/app/private/components/prism/prism.module';
import { RouterModule } from '@angular/router';
import { EventDeliveryDetailsComponent } from './event-delivery-details.component';
import { LoaderModule } from 'src/app/private/components/loader/loader.module';
import { CardComponent } from 'src/app/components/card/card.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { SkeletonLoaderComponent } from 'src/app/components/skeleton-loader/skeleton-loader.component';
import { StatusColorModule } from 'src/app/pipes/status-color/status-color.module';
import { TooltipComponent } from 'src/app/components/tooltip/tooltip.component';
@NgModule({
	declarations: [EventDeliveryDetailsComponent],
	imports: [CommonModule, RouterModule, PrismModule, LoaderModule, CardComponent, ButtonComponent, TagComponent, StatusColorModule, SkeletonLoaderComponent, TooltipComponent],
	exports: [EventDeliveryDetailsComponent]
})
export class EventDeliveryDetailsModule {}
