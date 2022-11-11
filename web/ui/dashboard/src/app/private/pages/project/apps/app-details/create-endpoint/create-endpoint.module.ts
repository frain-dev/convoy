import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Routes } from '@angular/router';
import { ReactiveFormsModule } from '@angular/forms';
import { CreateEndpointComponent } from './create-endpoint.component';
import { InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent } from 'src/app/components/input/input.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { RadioComponent } from 'src/app/components/radio/radio.component';
import { TooltipComponent } from 'src/app/components/tooltip/tooltip.component';

const routes: Routes = [
	{
		path: '',
		component: CreateEndpointComponent
	}
];
@NgModule({
	declarations: [CreateEndpointComponent],
	imports: [CommonModule, ReactiveFormsModule, ButtonComponent, RadioComponent, TooltipComponent, InputFieldDirective, InputErrorComponent, InputDirective, LabelComponent],
	exports: [CreateEndpointComponent]
})
export class CreateEndpointModule {}
