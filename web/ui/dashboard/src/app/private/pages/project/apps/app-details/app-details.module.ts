import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { AppDetailsComponent } from './app-details.component';
import { RouterModule, Routes } from '@angular/router';
import { CreateEndpointModule } from './create-endpoint/create-endpoint.module';
import { CardComponent } from 'src/app/components/card/card.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { ListItemComponent } from 'src/app/components/list-item/list-item.component';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { SkeletonLoaderComponent } from 'src/app/components/skeleton-loader/skeleton-loader.component';
import { SendEventComponent } from './send-event/send-event.component';
import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';
import { DeleteModalComponent } from 'src/app/private/components/delete-modal/delete-modal.component';
import { CliKeysComponent } from './cli-keys/cli-keys.component';
import { DevicesComponent } from './devices/devices.component';
import { TooltipComponent } from 'src/app/components/tooltip/tooltip.component';
import { SelectComponent } from 'src/app/components/select/select.component';
import { ReactiveFormsModule } from '@angular/forms';

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
		DeleteModalComponent,
		CardComponent,
		ButtonComponent,
		EmptyStateComponent,
		ListItemComponent,
		ModalComponent,
        EmptyStateComponent,
        SkeletonLoaderComponent,
		CreateEndpointModule,
        SendEventComponent,
		CopyButtonComponent,
        CliKeysComponent,
        DevicesComponent,
        TooltipComponent,
        SelectComponent,
        ReactiveFormsModule,
		RouterModule.forChild(routes)
	]
})
export class AppDetailsModule {}
