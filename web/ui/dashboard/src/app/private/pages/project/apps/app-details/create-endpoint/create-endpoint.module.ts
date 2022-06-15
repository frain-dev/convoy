import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Routes } from '@angular/router';
import { ReactiveFormsModule } from '@angular/forms';
import { TooltipModule } from 'src/app/private/components/tooltip/tooltip.module';
import { CreateEndpointComponent } from './create-endpoint.component';

const routes: Routes = [
	{
		path: '',
		component: CreateEndpointComponent
	}
];
@NgModule({
	declarations: [CreateEndpointComponent],
	imports: [CommonModule, ReactiveFormsModule, TooltipModule],
	exports: [CreateEndpointComponent]
})
export class CreateEndpointModule {}
