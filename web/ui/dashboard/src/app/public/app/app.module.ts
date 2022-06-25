import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { AppComponent } from './app.component';
import { RouterModule, Routes } from '@angular/router';
import { EventModule } from 'src/app/private/pages/project/events/event/event.module';
import { EventDeliveriesModule } from 'src/app/private/pages/project/events/event-deliveries/event-deliveries.module';
import { TableLoaderModule } from 'src/app/private/components/table-loader/table-loader.module';
import { CreateSubscriptionModule } from 'src/app/private/components/create-subscription/create-subscription.module';
import { CreateEndpointModule } from 'src/app/private/pages/project/apps/app-details/create-endpoint/create-endpoint.module';

const routes: Routes = [{ path: '', component: AppComponent }];

@NgModule({
	declarations: [AppComponent],
	imports: [CommonModule, RouterModule.forChild(routes), EventModule, EventDeliveriesModule, TableLoaderModule, CreateSubscriptionModule, CreateEndpointModule]
})
export class AppModule {}
