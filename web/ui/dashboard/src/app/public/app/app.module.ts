import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { AppComponent } from './app.component';
import { RouterModule, Routes } from '@angular/router';
import { EventDeliveriesModule } from 'src/app/private/pages/project/events/event-deliveries/event-deliveries.module';
import { TableLoaderModule } from 'src/app/private/components/table-loader/table-loader.module';
import { CreateSubscriptionModule } from 'src/app/private/components/create-subscription/create-subscription.module';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { PageComponent } from 'src/app/components/page/page.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { TableComponent, TableCellComponent, TableRowComponent, TableHeadCellComponent, TableHeadComponent } from 'src/app/components/table/table.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { ListItemComponent } from 'src/app/components/list-item/list-item.component';
import { StatusColorModule } from 'src/app/pipes/status-color/status-color.module';
import { DropdownComponent } from 'src/app/components/dropdown/dropdown.component';
import { DeleteModalComponent } from 'src/app/private/components/delete-modal/delete-modal.component';
import { CliKeysComponent } from 'src/app/private/pages/project/endpoint-details/cli-keys/cli-keys.component';
import { DevicesComponent } from 'src/app/private/pages/project/endpoint-details/devices/devices.component';
import { CreateEndpointComponent } from 'src/app/private/components/create-endpoint/create-endpoint.component';

const routes: Routes = [{ path: '', component: AppComponent }];

@NgModule({
	declarations: [AppComponent],
	imports: [
		CommonModule,
		RouterModule.forChild(routes),
		StatusColorModule,
		EventDeliveriesModule,
		TableLoaderModule,
		CreateSubscriptionModule,
		CreateEndpointComponent,
		DeleteModalComponent,
		ModalComponent,
		PageComponent,
		ButtonComponent,
		CardComponent,
		EmptyStateComponent,
		DropdownComponent,
		TagComponent,
		ListItemComponent,
		TableHeadComponent,
		TableRowComponent,
		TableCellComponent,
		TableHeadCellComponent,
		TableComponent,
		CliKeysComponent,
		DevicesComponent
	]
})
export class AppModule {}
