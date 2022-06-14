import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { AppDetailsComponent } from './app-details.component';
import { RouterModule, Routes } from '@angular/router';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { TooltipModule } from 'src/app/private/components/tooltip/tooltip.module';
import { CreateEndpointComponent } from './create-endpoint/create-endpoint.component';
import { SendEventComponent } from './send-event/send-event.component';

const routes: Routes = [
	{
		path: '',
		component: AppDetailsComponent
	}
];
@NgModule({
	declarations: [AppDetailsComponent, CreateEndpointComponent, SendEventComponent],
	imports: [CommonModule, ReactiveFormsModule, FormsModule, TooltipModule, RouterModule.forChild(routes)]
})
export class AppDetailsModule {}
