import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CreateAppComponent } from './create-app.component';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { TooltipModule } from '../tooltip/tooltip.module';

@NgModule({
	declarations: [CreateAppComponent],
	imports: [CommonModule, ReactiveFormsModule, FormsModule, TooltipModule],
	exports: [CreateAppComponent]
})
export class CreateAppModule {}
