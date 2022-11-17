import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { AppComponent } from './app.component';
import { RouterModule, Routes } from '@angular/router';
import { EventModule } from 'src/app/private/pages/project/events/event/event.module';
import { EventDeliveriesModule } from 'src/app/private/pages/project/events/event-deliveries/event-deliveries.module';
import { TableLoaderModule } from 'src/app/private/components/table-loader/table-loader.module';
import { CreateSubscriptionModule } from 'src/app/private/components/create-subscription/create-subscription.module';
import { CreateEndpointModule } from 'src/app/private/pages/project/apps/app-details/create-endpoint/create-endpoint.module';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { PageComponent } from 'src/app/components/page/page.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { TableHeadComponent } from 'src/app/components/table-head/table-head.component';
import { TableRowComponent } from 'src/app/components/table-row/table-row.component';
import { TableCellComponent } from 'src/app/components/table-cell/table-cell.component';
import { TableHeadCellComponent } from 'src/app/components/table-head-cell/table-head-cell.component';
import { TableComponent } from 'src/app/components/table/table.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { ListItemComponent } from 'src/app/components/list-item/list-item.component';
import { StatusColorModule } from 'src/app/pipes/status-color/status-color.module';
import { DropdownComponent } from 'src/app/components/dropdown/dropdown.component';
import { DeleteModalComponent } from 'src/app/private/components/delete-modal/delete-modal.component';
import { CliKeysComponent } from 'src/app/private/pages/project/endpoint-details/cli-keys/cli-keys.component';
import { DevicesComponent } from 'src/app/private/pages/project/endpoint-details/devices/devices.component';

const routes: Routes = [{ path: '', component: AppComponent }];

@NgModule({
	declarations: [AppComponent],
	imports: [
		CommonModule,
		RouterModule.forChild(routes),
		StatusColorModule,
		EventModule,
		EventDeliveriesModule,
		TableLoaderModule,
		CreateSubscriptionModule,
		CreateEndpointModule,
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
