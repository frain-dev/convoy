import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { AppDetailsComponent } from './app-details.component';
import { RouterModule, Routes } from '@angular/router';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { TooltipModule } from 'src/app/private/components/tooltip/tooltip.module';
import { SendEventComponent } from './send-event/send-event.component';
import { CreateEndpointModule } from './create-endpoint/create-endpoint.module';
import { DeleteModalModule } from 'src/app/private/components/delete-modal/delete-modal.module';

const routes: Routes = [
	{
		path: '',
		component: AppDetailsComponent
	}
];
@NgModule({
	declarations: [AppDetailsComponent, SendEventComponent],
	imports: [CommonModule, ReactiveFormsModule, FormsModule, TooltipModule, DeleteModalModule, RouterModule.forChild(routes), CreateEndpointModule]
})
export class AppDetailsModule {}
