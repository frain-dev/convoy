import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CreateSubscriptionComponent } from './create-subscription.component';
import { ReactiveFormsModule } from '@angular/forms';
import { CreateAppModule } from '../create-app/create-app.module';
import { CreateSourceModule } from '../create-source/create-source.module';
import { TooltipModule } from '../tooltip/tooltip.module';
import { LoaderModule } from '../loader/loader.module';
import { CreateEndpointModule } from '../../pages/project/apps/app-details/create-endpoint/create-endpoint.module';

@NgModule({
	declarations: [CreateSubscriptionComponent],
	imports: [CommonModule, ReactiveFormsModule, CreateAppModule, CreateSourceModule, TooltipModule, LoaderModule, CreateEndpointModule],
	exports: [CreateSubscriptionComponent]
})
export class CreateSubscriptionModule {}
