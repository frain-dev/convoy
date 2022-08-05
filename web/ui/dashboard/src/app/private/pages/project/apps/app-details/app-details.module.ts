import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { AppDetailsComponent } from './app-details.component';
import { RouterModule, Routes } from '@angular/router';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { SendEventComponent } from './send-event/send-event.component';
import { CreateEndpointModule } from './create-endpoint/create-endpoint.module';
import { DeleteModalModule } from 'src/app/private/components/delete-modal/delete-modal.module';
import { CardComponent } from 'src/app/components/card/card.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { ListItemComponent } from 'src/app/components/list-item/list-item.component';
import { InputComponent } from 'src/app/components/input/input.component';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { SelectComponent } from 'src/app/components/select/select.component';
import { TooltipComponent } from 'src/app/components/tooltip/tooltip.component';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { PipesModule } from 'src/app/pipes/pipes.module';
import { CliComponent } from './cli/cli.component';
import { SkeletonLoaderComponent } from 'src/app/components/skeleton-loader/skeleton-loader.component';

const routes: Routes = [
	{
		path: '',
		component: AppDetailsComponent
	}
];
@NgModule({
	declarations: [AppDetailsComponent, SendEventComponent, CliComponent],
	imports: [
		CommonModule,
		ReactiveFormsModule,
		FormsModule,
		DeleteModalModule,
		CardComponent,
		ButtonComponent,
		EmptyStateComponent,
		ListItemComponent,
		InputComponent,
		SelectComponent,
		ModalComponent,
		TooltipComponent,
		CreateEndpointModule,
		TagComponent,
        PipesModule,
        EmptyStateComponent,
        SkeletonLoaderComponent,
		RouterModule.forChild(routes)
	]
})
export class AppDetailsModule {}
