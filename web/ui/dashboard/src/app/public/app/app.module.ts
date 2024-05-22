import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { AppComponent } from './app.component';
import { RouterModule, Routes } from '@angular/router';
import { EventDeliveriesModule } from 'src/app/private/pages/project/events/event-deliveries/event-deliveries.module';
import { CreateSubscriptionModule } from 'src/app/private/components/create-subscription/create-subscription.module';
import { PageDirective } from 'src/app/components/page/page.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { DropdownComponent, DropdownOptionDirective } from 'src/app/components/dropdown/dropdown.component';
import { CreateEndpointComponent } from 'src/app/private/components/create-endpoint/create-endpoint.component';
import { CreatePortalEndpointComponent } from '../create-endpoint/create-endpoint.component';
import { EndpointSecretComponent } from 'src/app/private/pages/project/endpoints/endpoint-secret/endpoint-secret.component';
import { DialogDirective } from 'src/app/components/dialog/dialog.directive';
import { SubscriptionsComponent } from '../subscriptions/subscriptions.component';
import { StatusColorModule } from 'src/app/pipes/status-color/status-color.module';

const routes: Routes = [{ path: '', component: AppComponent }];

@NgModule({
	declarations: [AppComponent],
	imports: [
		CommonModule,
		RouterModule.forChild(routes),
		EventDeliveriesModule,
		CreateSubscriptionModule,
		CreateEndpointComponent,
		PageDirective,
		ButtonComponent,
		CardComponent,
		EmptyStateComponent,
		DropdownComponent,
		DropdownOptionDirective,
		TagComponent,
		EndpointSecretComponent,
		CreatePortalEndpointComponent,
        DialogDirective,
        SubscriptionsComponent,
        StatusColorModule
	]
})
export class AppModule {}
