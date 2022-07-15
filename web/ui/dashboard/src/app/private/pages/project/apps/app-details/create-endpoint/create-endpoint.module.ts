import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Routes } from '@angular/router';
import { ReactiveFormsModule } from '@angular/forms';
import { CreateEndpointComponent } from './create-endpoint.component';
import { InputComponent } from 'src/app/components/input/input.component';
import { ButtonComponent } from 'src/app/components/button/button.component';

const routes: Routes = [
	{
		path: '',
		component: CreateEndpointComponent
	}
];
@NgModule({
	declarations: [CreateEndpointComponent],
	imports: [CommonModule, ReactiveFormsModule, InputComponent, ButtonComponent],
	exports: [CreateEndpointComponent]
})
export class CreateEndpointModule {}
