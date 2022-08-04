import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { PrismModule } from 'src/app/private/components/prism/prism.module';
import { RouterModule } from '@angular/router';
import { EventDeliveryDetailsComponent } from './event-delivery-details.component';
import { LoaderModule } from 'src/app/private/components/loader/loader.module';
import { CardComponent } from 'src/app/components/card/card.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { PipesModule } from 'src/app/pipes/pipes.module';
import { SkeletonLoaderComponent } from 'src/app/components/skeleton-loader/skeleton-loader.component';
import { ModalComponent } from 'src/app/components/modal/modal.component';

@NgModule({
	declarations: [EventDeliveryDetailsComponent],
	imports: [CommonModule, RouterModule, PrismModule, LoaderModule, CardComponent, ButtonComponent, TagComponent, PipesModule, SkeletonLoaderComponent, ModalComponent],
	exports: [EventDeliveryDetailsComponent]
})
export class EventDeliveryDetailsModule {}
