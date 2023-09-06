import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { PrismModule } from 'src/app/private/components/prism/prism.module';
import { RouterModule } from '@angular/router';
import { EventDeliveryDetailsComponent } from './event-delivery-details.component';
import { LoaderModule } from 'src/app/private/components/loader/loader.module';
import { CardComponent } from 'src/app/components/card/card.component';
import { ButtonComponent, ButtonGroupDirective } from 'src/app/components/button/button.component';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { SkeletonLoaderComponent } from 'src/app/components/skeleton-loader/skeleton-loader.component';
import { StatusColorModule } from 'src/app/pipes/status-color/status-color.module';
import { DropdownComponent, DropdownOptionDirective } from 'src/app/components/dropdown/dropdown.component';
@NgModule({
	declarations: [EventDeliveryDetailsComponent],
	imports: [CommonModule, RouterModule, PrismModule, LoaderModule, CardComponent, ButtonComponent, ButtonGroupDirective, TagComponent, StatusColorModule, SkeletonLoaderComponent, DropdownComponent, DropdownOptionDirective],
	exports: [EventDeliveryDetailsComponent]
})
export class EventDeliveryDetailsModule {}
