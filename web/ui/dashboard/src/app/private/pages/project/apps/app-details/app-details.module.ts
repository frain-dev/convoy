import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { AppDetailsComponent } from './app-details.component';
import { RouterModule, Routes } from '@angular/router';
import { CreateEndpointModule } from './create-endpoint/create-endpoint.module';
import { DeleteModalModule } from 'src/app/private/components/delete-modal/delete-modal.module';
import { CardComponent } from 'src/app/components/card/card.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { ListItemComponent } from 'src/app/components/list-item/list-item.component';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { SkeletonLoaderComponent } from 'src/app/components/skeleton-loader/skeleton-loader.component';
import { CliComponent } from './cli/cli.component';
import { SendEventComponent } from './send-event/send-event.component';

const routes: Routes = [
	{
		path: '',
		component: AppDetailsComponent
	}
];
@NgModule({
	declarations: [AppDetailsComponent],
	imports: [
		CommonModule,
		DeleteModalModule,
		CardComponent,
		ButtonComponent,
		EmptyStateComponent,
		ListItemComponent,
		ModalComponent,
        EmptyStateComponent,
        SkeletonLoaderComponent,
		CreateEndpointModule,
        SendEventComponent,
        CliComponent,
		RouterModule.forChild(routes)
	]
})
export class AppDetailsModule {}
